package app

import (
	"github.com/glebarez/sqlite"
	sloggorm "github.com/imdatngo/slog-gorm"
	"github.com/jovandeginste/spark-personal-assistant/pkg/data"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type DatabaseConfig struct {
	File         string `mapstructure:"file"`
	originalFile string
}

func (a *App) DB() *gorm.DB {
	return a.db.Preload(clause.Associations)
}

func (a *App) Migrate() error {
	return a.db.AutoMigrate(
		data.Source{}, data.Entry{},
	)
}

func (a *App) initializeDatabase() error {
	c := &gorm.Config{
		Logger: sloggorm.NewWithConfig(sloggorm.NewConfig(a.Logger().Handler())),
	}

	db, err := gorm.Open(sqlite.Open(a.Config.Database.File), c)
	if err != nil {
		return err
	}

	a.db = db

	return a.Migrate()
}
