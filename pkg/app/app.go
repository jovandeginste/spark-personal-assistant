package app

import (
	"log/slog"

	"gorm.io/gorm"
)

type App struct {
	ConfigFile string
	Config     Config

	db     *gorm.DB
	logger slog.Logger
}

func NewApp() *App {
	a := &App{}

	return a
}

func (a *App) Logger() *slog.Logger {
	return &a.logger
}

func (a *App) Initialize() error {
	if err := a.ReadConfig(); err != nil {
		return err
	}

	a.Config.Mailer.app = a

	a.initializeLogger()

	if err := a.initializeDatabase(); err != nil {
		return err
	}

	return nil
}

func (a *App) initializeLogger() {
	a.logger = *slog.Default()
}
