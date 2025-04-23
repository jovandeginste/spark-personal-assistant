package app

import (
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
	"gorm.io/gorm"
)

type Config struct {
	Database     DatabaseConfig `mapstructure:"database"`
	EmployerData EmployerData   `mapstructure:"employer_data"`
	Mailer       Mailer         `mapstructure:"mail"`
}

type EmployerData struct {
	Names []string `mapstructure:"names"`
}

type App struct {
	ConfigFile string
	Config     Config

	db     *gorm.DB
	logger slog.Logger
}

func (a *App) Logger() *slog.Logger {
	return &a.logger
}

func NewApp() *App {
	a := &App{}

	return a
}

func (a *App) ReadConfig() error {
	viper.SetConfigFile(a.ConfigFile)

	if err := viper.ReadInConfig(); err != nil { // Handle errors reading the config file
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return err
		}
	}

	if err := viper.Unmarshal(&a.Config); err != nil {
		return err
	}

	if strings.HasPrefix(a.Config.Database.File, "/") {
		return nil
	}

	absPath, err := filepath.Abs(a.ConfigFile)
	if err != nil {
		return err
	}

	dirname := filepath.Dir(absPath)
	a.Config.Database.File = filepath.Clean(filepath.Join(dirname, a.Config.Database.File))

	return nil
}

func (a *App) initializeLogger() {
	a.logger = *slog.Default()
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
