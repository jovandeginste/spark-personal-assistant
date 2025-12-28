package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/adrg/frontmatter"
	"github.com/jovandeginste/spark-personal-assistant/personas"
	"github.com/jovandeginste/spark-personal-assistant/pkg/ai"
	"github.com/spf13/viper"
)

var defaultPersona = "butler.md"

type Config struct {
	Database     DatabaseConfig `mapstructure:"database"`
	UserData     UserData       `mapstructure:"user_data"`
	ExtraContext []string       `mapstructure:"extra_context"`
	Mailer       Mailer         `mapstructure:"mail"`
	LLM          *ai.AIConfig   `mapstructure:"llm"`
	TTS          *TTSConfig     `mapstructure:"tts"`

	AssistantFileCLI string             `mapstructure:"-"`
	Assistant        ai.AssistantConfig `mapstructure:"assistant"`
	Matrix           MatrixConfig       `mapstructure:"matrix"`
	Webserver        WebserverConfig    `mapstructure:"webserver"`
}

type TTSConfig struct {
	Type   string `mapstructure:"type"`
	Voice  string `mapstructure:"voice"`
	Lang   string `mapstructure:"lang"`
	APIKey string `mapstructure:"api_key"`
}

type MatrixConfig struct {
	Homeserver string   `mapstructure:"homeserver"`
	Username   string   `mapstructure:"username"`
	Password   string   `mapstructure:"password"`
	RoomID     string   `mapstructure:"room_id"`
	Database   string   `mapstructure:"database"`
	Users      []string `mapstructure:"users"`
}

type WebserverConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Bind    string `mapstructure:"bind"`
}

type UserData struct {
	Names []string `mapstructure:"names"`
}

func (a *App) ReadConfig() error {
	viper.SetConfigFile(a.ConfigFile)

	if err := viper.ReadInConfig(); err != nil { // Handle errors reading the config file
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}

	if err := viper.Unmarshal(&a.Config); err != nil {
		return err
	}

	for _, f := range []func() error{
		a.setAssistantStylePath,
		a.setMatrixDatabasePath,
		a.configureAssistant,
	} {
		if err := f(); err != nil {
			return err
		}
	}

	a.SetDefaults()

	a.Config.Database.originalFile = a.Config.Database.File

	return a.setDatabasePath()
}

func (a *App) configureAssistant() error {
	if a.Config.Assistant.File == "" {
		return nil
	}

	input, err := os.Open(a.Config.Assistant.File)
	if err != nil {
		a.Logger().Error("Could not open assistant file", "file", a.Config.Assistant.File, "error", err.Error())
		return nil
	}

	var assistantFromFile ai.AssistantConfig

	rest, err := frontmatter.Parse(input, &assistantFromFile)
	if err != nil {
		return err
	}

	if a.Config.Assistant.Name == "" {
		a.Config.Assistant.Name = assistantFromFile.Name
	}

	if a.Config.Assistant.Language == "" {
		a.Config.Assistant.Language = assistantFromFile.Language
	}

	if a.Config.Assistant.Style == "" {
		a.Config.Assistant.Style = string(rest)
	}

	return nil
}

func (a *App) setMatrixDatabasePath() error {
	if a.Config.Matrix.Database == "" || strings.HasPrefix(a.Config.Matrix.Database, "/") {
		return nil
	}

	absPath, err := filepath.Abs(a.ConfigFile)
	if err != nil {
		return err
	}

	dirname := filepath.Dir(absPath)
	a.Config.Matrix.Database = filepath.Join(filepath.Clean(dirname), filepath.Clean(a.Config.Matrix.Database))

	return nil
}

func (a *App) setAssistantStylePath() error {
	if a.Config.AssistantFileCLI != "" {
		a.Config.Assistant.File = a.Config.AssistantFileCLI
	}

	if a.Config.Assistant.File == "" || strings.HasPrefix(a.Config.Assistant.File, "/") {
		return nil
	}

	absPath, err := filepath.Abs(a.ConfigFile)
	if err != nil {
		return err
	}

	dirname := filepath.Dir(absPath)
	a.Config.Assistant.File = filepath.Join(filepath.Clean(dirname), filepath.Clean(a.Config.Assistant.File))

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

func readPersona(persona string) *ai.AssistantConfig {
	var assistantFromFile ai.AssistantConfig

	if !strings.HasSuffix(persona, ".md") {
		persona += ".md"
	}

	data, err := personas.FS().Open(persona)
	if err != nil {
		return nil
	}

	rest, err := frontmatter.Parse(data, &assistantFromFile)
	if err != nil {
		return nil
	}

	assistantFromFile.Style = string(rest)

	return &assistantFromFile
}

func (a *App) SetDefaults() {
	if a.Config.Webserver.Bind == "" {
		a.Config.Webserver.Bind = ":8080"
	}

	if a.Config.Database.File == "" {
		a.Config.Database.File = "spark.db"
	}

	if a.Config.Assistant.Language == "" {
		a.Config.Assistant.Language = "English"
	}

	a.setDefaultPersona()
}

func (a *App) setDefaultPersona() {
	dp := readPersona(defaultPersona)
	if dp == nil {
		return
	}

	if a.Config.Assistant.Name == "" {
		a.Config.Assistant.Name = dp.Name
	}

	if a.Config.Assistant.Style == "" {
		a.Config.Assistant.Style = dp.Style
	}
}

func (a *App) SetPersona(persona string) {
	dp := readPersona(persona)
	if dp == nil {
		return
	}

	a.Config.Assistant.Name = dp.Name
	a.Config.Assistant.Style = dp.Style
}
