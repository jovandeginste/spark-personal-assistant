package app

import (
	"github.com/glebarez/sqlite"
	sloggorm "github.com/imdatngo/slog-gorm"
	"github.com/jovandeginste/spark-personal-assistant/pkg/structs"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type DatabaseConfig struct {
	File           string `mapstructure:"file"`
	InternalSource string `mapstructure:"internal_source"`
	originalFile   string
}

func (a *App) DB() *gorm.DB {
	return a.db.Preload(clause.Associations)
}

func (a *App) Query() *gorm.DB {
	return a.DB().Session(&gorm.Session{}).Debug()
}

func (a *App) Migrate() error {
	a.Logger().Info("Migrating database")

	err := a.db.AutoMigrate(structs.Source{}, structs.Entry{})
	if err != nil {
		return err
	}

	return a.db.Migrator().DropColumn(&structs.Entry{}, "importance")
}

func (a *App) initializeDatabase() error {
	c := &gorm.Config{
		Logger: sloggorm.NewWithConfig(sloggorm.NewConfig(
			a.Logger().With("component", "gorm").Handler(),
		)),
	}

	db, err := gorm.Open(sqlite.Open(a.Config.Database.File), c)
	if err != nil {
		return err
	}

	a.db = db

	return a.Migrate()
}
