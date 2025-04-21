# slog-gorm

[![Release](https://img.shields.io/github/v/release/imdatngo/slog-gorm)](https://github.com/imdatngo/slog-gorm/releases)
![Go version](https://img.shields.io/github/go-mod/go-version/imdatngo/slog-gorm)
[![Go Reference](https://pkg.go.dev/badge/github.com/imdatngo/slog-gorm.svg)](https://pkg.go.dev/github.com/imdatngo/slog-gorm)
![Tests](https://github.com/imdatngo/slog-gorm/actions/workflows/tests.yml/badge.svg)
[![Go report](https://goreportcard.com/badge/github.com/imdatngo/slog-gorm)](https://goreportcard.com/report/github.com/imdatngo/slog-gorm)
[![codecov](https://codecov.io/gh/imdatngo/slog-gorm/graph/badge.svg?token=KM0Y198PUH)](https://codecov.io/gh/imdatngo/slog-gorm)
[![License](https://img.shields.io/github/license/imdatngo/slog-gorm)](./LICENSE)

slog handler for [Gorm](https://github.com/go-gorm/gorm), inspired by [orandin/slog-gorm](https://github.com/orandin/slog-gorm) with my own ideas to tailor it to my specific needs.

## 🚀 Install

```sh
go get github.com/imdatngo/slog-gorm
```

**Compatibility**: go >= 1.21

## 💡 Usage

### Minimal

See [config.go](./config.go) for default values.

```go
import (
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	sloggorm "github.com/imdatngo/slog-gorm"
)

// Create new slog-gorm instance with slog.Default()
glogger := sloggorm.New()

// Globally mode
db, err := gorm.Open(sqlite.Open("test.db"), &gorm.Config{
	Logger: glogger,
})

// Continuous session mode
tx := db.Session(&gorm.Session{Logger: glogger})
tx.First(&user)
tx.Model(&user).Update("Age", 18)

// Sample output:
// 2024/04/16 07:30:00 ERROR Query ERROR duration=128.364µs rows=0 file=main.go:45 error="record not found" query="SELECT * FROM `users` ORDER BY `users`.`id` LIMIT 1"
// 2024/04/16 07:30:00 WARN Query SLOW duration=133.448µs rows=0 file=main.go:46 slow_threshold=100ns query="UPDATE `users` SET `age`=18 WHERE `id` = 1"
```

### With custom config

```go
// Your slog.Logger instance
slogger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
// with context field and/or group to distinguish with application logs
slogger = slogger.With(slog.Any("logger", "db"))
// slogger = slogger.WithGroup("db")

// Create new slog-gorm instance with custom config
cfg := sloggorm.NewConfig(slogger.Handler()).
	WithSlowThreshold(time.Second).
	WithIgnoreRecordNotFoundError(true).
	WithTraceAll(true).
	WithContextKeys(map[string]string{"req_id": "X-Request-ID"})
glogger := sloggorm.NewWithConfig(cfg)

// Sample output:
// time=2024-04-16T07:35:40.696Z level=INFO msg="Query OK" logger=db req_id=01ARZ3NDEKTSV4RRFFQ69G5FAV duration=130.659µs rows=1 file=main.go:45 query="SELECT * FROM `users` WHERE `users`.`id` = 1 ORDER BY `users`.`id` LIMIT 1"
// time=2024-04-16T07:35:40.697Z level=INFO msg="Query OK" logger=db req_id=01ARZ3NDEKTSV4RRFFQ69G5FAV duration=940.445µs rows=1 file=main.go:46 query="UPDATE `users` SET `age`=18 WHERE `id` = 1"
```

### Silence!

The slow queries and errors are logged by default, to discard all logs:

```go
cfg := sloggorm.NewConfig(slogger.Handler()).WithSilent(true)
glogger := sloggorm.NewWithConfig(cfg)
```

To on/off in session mode:

```go
// Start gorm's debug mode which is equivalent to cfg.WithTraceAll(true)
db.Debug().First(&User{})
// similar to new session
newLogger := glogger.LogMode(gormlogger.Info)
tx := db.Session(&gorm.Session{Logger: newLogger})

// or discard all logs for a session
tx := db.Session(&gorm.Session{Logger: db.Logger.LogMode(gormlogger.Silent)})
```
