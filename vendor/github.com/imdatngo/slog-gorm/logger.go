package sloggorm

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path"
	"runtime"
	"time"

	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
	"gorm.io/gorm/utils"
)

// New creates a new logger with default config
func New() *logger {
	return NewWithConfig(NewConfig(slog.Default().Handler()))
}

// NewWithConfig creates a new logger with given config
func NewWithConfig(config *config) *logger {
	return &logger{
		config,
	}
}

type logger struct {
	*config
}

// ensure our logger implements gormlogger.Interface
var _ gormlogger.Interface = (*logger)(nil)

// LogMode log mode
func (l *logger) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	// This function will switch to logging all queries, whenever the level is set to Info.
	// It's to support the Debug() function of gorm which sets the log level to info for subsequent queries, see:
	//   https://gorm.io/docs/session.html#Debug

	// Note: Error and Warn levels are ignored as the log level is managed by slog already.
	if level == gormlogger.Error || level == gormlogger.Warn {
		return l
	}

	// clone logger for session mode
	nc := l.config.clone()
	nl := NewWithConfig(nc.WithTraceAll(level == gormlogger.Info).WithSilent(level == gormlogger.Silent))
	return nl
}

// Info logs info message
func (l *logger) Info(ctx context.Context, format string, args ...any) {
	if l.enabled(ctx, slog.LevelInfo) {
		l.log(ctx, slog.LevelInfo, fmt.Sprintf(format, args...), l.contextAttrs(ctx)...)
	}
}

// Warn logs warn message
func (l *logger) Warn(ctx context.Context, format string, args ...any) {
	if l.enabled(ctx, slog.LevelWarn) {
		l.log(ctx, slog.LevelWarn, fmt.Sprintf(format, args...), l.contextAttrs(ctx)...)
	}
}

// Error logs error message
func (l *logger) Error(ctx context.Context, format string, args ...any) {
	if l.enabled(ctx, slog.LevelError) {
		l.log(ctx, slog.LevelError, fmt.Sprintf(format, args...), l.contextAttrs(ctx)...)
	}
}

// Trace logs sql message
func (l *logger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if l.silent {
		return
	}

	elapsed := time.Since(begin)
	switch {
	case err != nil && l.enabled(ctx, slog.LevelError) && (!errors.Is(err, gorm.ErrRecordNotFound) || !l.ignoreRecordNotFoundError):
		attrs := l.traceAttrs(ctx, elapsed, fc, utils.FileWithLineNum(), err, false)
		l.log(ctx, slog.LevelError, "Query ERROR", attrs...)
	case l.slowThreshold != 0 && elapsed > l.slowThreshold && l.enabled(ctx, slog.LevelWarn):
		attrs := l.traceAttrs(ctx, elapsed, fc, utils.FileWithLineNum(), err, true)
		l.log(ctx, slog.LevelWarn, "Query SLOW", attrs...)
	case l.traceAll && l.enabled(ctx, slog.LevelInfo):
		attrs := l.traceAttrs(ctx, elapsed, fc, utils.FileWithLineNum(), err, false)
		l.log(ctx, slog.LevelInfo, "Query OK", attrs...)
	}
}

// ParamsFilter filter params
func (l *logger) ParamsFilter(_ context.Context, sql string, params ...interface{}) (string, []interface{}) {
	if l.parameterizedQueries {
		return sql, nil
	}
	return sql, params
}

// log adds context attributes and logs a message with the given slog level
func (l *logger) log(ctx context.Context, level slog.Level, msg string, attrs ...any) {
	var pcs [1]uintptr
	// skip [runtime.Callers, this function, this function's caller]
	runtime.Callers(3, pcs[:])
	r := slog.NewRecord(time.Now(), level, msg, pcs[0])
	r.Add(attrs...)

	if ctx == nil {
		ctx = context.Background()
	}
	_ = l.slogHandler.Handle(ctx, r)
}

func (l *logger) traceAttrs(ctx context.Context, elapsed time.Duration, fc func() (string, int64), file string, err error, slow bool) []any {
	sql, rows := fc()

	attrs := make([]any, 0, len(l.contextKeys)+5) // max 5 fixed attrs
	attrs = append(attrs, l.contextAttrs(ctx)...)

	if l.durationField != "" {
		attrs = append(attrs, slog.Duration(l.durationField, elapsed))
	}
	if rows >= 0 && l.rowsField != "" { // rows could be -1
		attrs = append(attrs, slog.Int64(l.rowsField, rows))
	}
	if l.sourceField != "" {
		if l.fullSourcePath {
			attrs = append(attrs, slog.String(l.sourceField, file))
		} else {
			attrs = append(attrs, slog.String(l.sourceField, path.Base(file)))
		}
	}
	if err != nil && l.errorField != "" {
		attrs = append(attrs, slog.Any(l.errorField, err))
	} else if slow && l.slowThresholdField != "" {
		attrs = append(attrs, slog.Duration(l.slowThresholdField, l.slowThreshold))
	}
	if l.queryField != "" { // really?
		attrs = append(attrs, slog.String(l.queryField, sql))
	}

	return attrs
}

// contextAttrs extracts attributes from context
func (l *logger) contextAttrs(ctx context.Context) []any {
	if ctx == nil {
		ctx = context.Background()
	}
	attrs := make([]any, 0, len(l.contextKeys))
	for ak, cv := range l.contextKeys {
		if val := ctx.Value(cv); val != nil {
			attrs = append(attrs, slog.Any(ak, val))
		}
	}
	return attrs
}

// enabled reports whether the logger is enabled at the given level
func (l *logger) enabled(ctx context.Context, lvl slog.Level) bool {
	return !l.silent && l.slogHandler.Enabled(ctx, lvl)
}
