package app

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/adrg/frontmatter"
	"github.com/jovandeginste/spark-personal-assistant/pkg/ai"
	"github.com/spf13/viper"
)

type Config struct {
	AssistantFile string         `mapstructure:"assistant"`
	Database      DatabaseConfig `mapstructure:"database"`
	UserData      UserData       `mapstructure:"user_data"`
	ExtraContext  []string       `mapstructure:"extra_context"`
	Mailer        Mailer         `mapstructure:"mail"`
	LLM           *ai.AIConfig   `mapstructure:"llm"`

	AssistantFileCLI string             `mapstructure:"-"`
	Assistant        ai.AssistantConfig `mapstructure:"-"`
}

type UserData struct {
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

	if err := a.setAssistantStylePath(); err != nil {
		return err
	}

	if err := a.configureAssistant(); err != nil {
		return err
	}

	a.SetDefaults()

	a.Config.Database.originalFile = a.Config.Database.File

	return a.setDatabasePath()
}

func (a *App) configureAssistant() error {
	input, err := os.Open(a.Config.AssistantFile)
	if err != nil {
		return err
	}

	rest, err := frontmatter.Parse(input, &a.Config.Assistant)
	if err != nil {
		return err
	}

	a.Config.Assistant.Style = string(rest)

	return nil
}

func (a *App) setAssistantStylePath() error {
	if a.Config.AssistantFileCLI != "" {
		a.Config.AssistantFile = a.Config.AssistantFileCLI
	}

	if strings.HasPrefix(a.Config.AssistantFile, "/") {
		return nil
	}

	absPath, err := filepath.Abs(a.ConfigFile)
	if err != nil {
		return err
	}

	dirname := filepath.Dir(absPath)
	a.Config.AssistantFile = filepath.Join(filepath.Clean(dirname), filepath.Clean(a.Config.AssistantFile))

	return nil
}

func (a *App) setDatabasePath() error {
	if strings.HasPrefix(a.Config.Database.File, "/") {
		return nil
	}

	absPath, err := filepath.Abs(a.ConfigFile)
	if err != nil {
		return err
	}

	dirname := filepath.Dir(absPath)
	a.Config.Database.File = filepath.Join(filepath.Clean(dirname), filepath.Clean(a.Config.Database.File))

	return nil
}

func (a *App) SetDefaults() {
	if a.Config.Database.File == "" {
		a.Config.Database.File = "spark.db"
	}

	if a.Config.Assistant.Name == "" {
		a.Config.Assistant.Name = "Spark"
	}

	if a.Config.Assistant.Style == "" {
		a.Config.Assistant.Style = `You are the family butler and use polite British style and accent.
Be concise with chearful and colourful language.
Use conversational style.
Use emojis.`
	}
}
