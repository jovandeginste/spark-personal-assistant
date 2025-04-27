package app

import (
	"path/filepath"
	"strings"

	"github.com/jovandeginste/spark-personal-assistant/pkg/ai"
	"github.com/spf13/viper"
)

type Config struct {
	Assistant    ai.AssistantConfig `mapstructure:"assistant"`
	Database     DatabaseConfig     `mapstructure:"database"`
	EmployerData EmployerData       `mapstructure:"employer_data"`
	ExtraContext []string           `mapstructure:"extra_context"`
	Mailer       Mailer             `mapstructure:"mail"`
	LLM          *ai.AIConfig       `mapstructure:"llm"`
}

type EmployerData struct {
	Names []string `mapstructure:"names"`
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

	a.SetDefaults()

	absPath, err := filepath.Abs(a.ConfigFile)
	if err != nil {
		return err
	}

	dirname := filepath.Dir(absPath)
	a.Config.Database.File = filepath.Clean(filepath.Join(dirname, a.Config.Database.File))

	return nil
}

func (a *App) SetDefaults() {
	if a.Config.Assistant.Name == "" {
		a.Config.Assistant.Name = "Spark"
	}

	if a.Config.Assistant.Style == "" {
		a.Config.Assistant.Style = "polite British style and accent"
	}
}
