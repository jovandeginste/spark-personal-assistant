package app

import (
	"log/slog"
	"os"

	"github.com/spf13/viper"
	"gorm.io/gorm"
)

type Config struct {
	Database     DatabaseConfig `mapstructure:"database"`
	EmployerData EmployerData   `mapstructure:"employer_data"`
	Mailer       Mailer         `mapstructure:"mail"`
}

type App struct {
	Config Config

	ai     *Client
	db     *gorm.DB
	logger slog.Logger
}

func (a *App) Logger() *slog.Logger {
	return &a.logger
}

func NewApp() *App {
	a := &App{}

	a.ReadConfig()

	a.Config.Mailer.app = a

	return a
}

func (a *App) ReadConfig() error {
	viper.SetConfigName("spark")
	viper.AddConfigPath(".")

	if err := viper.ReadInConfig(); err != nil { // Handle errors reading the config file
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return err
		}
	}

	if err := viper.Unmarshal(&a.Config); err != nil {
		return err
	}

	return nil
}

func (a *App) initializeLogger() {
	a.logger = *slog.Default()
}

func (a *App) Initialize() error {
	a.initializeLogger()

	if err := a.initializeClient(); err != nil {
		return err
	}

	if err := a.initializeDatabase(); err != nil {
		return err
	}

	return nil
}

func (a *App) initializeClient() error {
	c, err := NewClient(os.Getenv("GEMINI_API_KEY"))
	if err != nil {
		return err
	}

	a.ai = c

	return nil
}
