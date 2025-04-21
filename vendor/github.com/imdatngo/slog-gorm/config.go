package sloggorm

import (
	"log/slog"
	"time"
)

// NewConfig creates a new config with the given non-nil slog.Handler
func NewConfig(h slog.Handler) *config {
	if h == nil {
		panic("nil Handler")
	}
	return &config{
		slogHandler:               h,
		slowThreshold:             200 * time.Millisecond,
		ignoreRecordNotFoundError: false,
		parameterizedQueries:      false,
		silent:                    false,
		traceAll:                  false,
		contextKeys:               map[string]string{},
		errorField:                "error",
		slowThresholdField:        "slow_threshold",
		queryField:                "query",
		durationField:             "duration",
		rowsField:                 "rows",
		sourceField:               "file",
		fullSourcePath:            false,
	}
}

// logger config
type config struct {
	slogHandler slog.Handler

	slowThreshold             time.Duration
	ignoreRecordNotFoundError bool
	parameterizedQueries      bool
	silent                    bool
	traceAll                  bool
	contextKeys               map[string]string

	errorField         string
	slowThresholdField string
	queryField         string
	durationField      string
	rowsField          string
	sourceField        string
	fullSourcePath     bool
}

// clone returns a new config with same values
func (c *config) clone() *config {
	nc := *c
	nc.contextKeys = map[string]string{}
	for k, v := range c.contextKeys {
		nc.contextKeys[k] = v
	}
	return &nc
}

// WithSlowThreshold sets slow SQL threshold. Default 200ms
func (c *config) WithSlowThreshold(v time.Duration) *config {
	c.slowThreshold = v
	return c
}

// WithIgnoreRecordNotFoundError whether to skip ErrRecordNotFound error
func (c *config) WithIgnoreRecordNotFoundError(v bool) *config {
	c.ignoreRecordNotFoundError = v
	return c
}

// WithParameterizedQueries whether to include params in the SQL log
func (c *config) WithParameterizedQueries(v bool) *config {
	c.parameterizedQueries = v
	return c
}

// WithSilent whether to discard all logs
func (c *config) WithSilent(v bool) *config {
	c.silent = v
	return c
}

// WithTraceAll whether to include OK queries in logs
func (c *config) WithTraceAll(v bool) *config {
	c.traceAll = v
	return c
}

// WithContextKeys includes additional log fields from context
func (c *config) WithContextKeys(v map[string]string) *config {
	for k, v := range v {
		c.contextKeys[k] = v
	}
	return c
}

// WithErrorField set attribute name for error field. Default "error"
func (c *config) WithErrorField(v string) *config {
	c.errorField = v
	return c
}

// WithSlowThresholdField changes attribute name of slow threshold field. Default "slow_threshold"
func (c *config) WithSlowThresholdField(v string) *config {
	c.slowThresholdField = v
	return c
}

// WithQueryField changes attribute name of SQL query field. Default "query"
func (c *config) WithQueryField(v string) *config {
	c.queryField = v
	return c
}

// WithDurationField changes attribute name of duration field. Default "duration"
func (c *config) WithDurationField(v string) *config {
	c.durationField = v
	return c
}

// WithRowsField changes attribute name of rows affected field. Default "rows"
func (c *config) WithRowsField(v string) *config {
	c.rowsField = v
	return c
}

// WithSourceField changes attribute name of source field. Default "file"
func (c *config) WithSourceField(v string) *config {
	c.sourceField = v
	return c
}

// WithFullSourcePath whether to include full path in source field or just the file name. Default false
func (c *config) WithFullSourcePath(v bool) *config {
	c.fullSourcePath = v
	return c
}
