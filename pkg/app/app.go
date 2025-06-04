package app

import (
	"log/slog"
	"os"

	"gorm.io/gorm"
)

type App struct {
	ConfigFile string
	Config     Config

	db     *gorm.DB
	logger *slog.Logger
}

func NewApp() *App {
	a := &App{}

	return a
}

func (a *App) Logger() *slog.Logger {
	return a.logger
}

func (a *App) Initialize() error {
	a.initializeLogger()

	if err := a.ReadConfig(); err != nil {
		return err
	}

	a.Config.Mailer.app = a

	if err := a.initializeDatabase(); err != nil {
		return err
	}

	return nil
}

func (a *App) initializeLogger() {
	a.logger = slog.New(slog.NewJSONHandler(os.Stderr, nil))
}

func (a *App) UpdateSources() error {
	sources, err := a.Sources()
	if err != nil {
		return err
	}

	for _, src := range sources {
		a.Logger().Info("Updating entries", "source", src.Name)

		entries, err := src.RetrieveEntries()
		if err != nil {
			return err
		}

		if len(entries) == 0 {
			a.Logger().Info("No entries found", "source", src.Name)
			return nil
		}

		a.Logger().Info("Entries retrieved", "source", src.Name, "count", len(entries))

		a.FetchExistingEntries(src.ID, entries)

		if err := a.ReplaceSourceEntries(&src, entries); err != nil {
			return err
		}

		a.Logger().Info("Entries updated", "source", src.Name, "count", len(entries))
	}

	return nil
}
