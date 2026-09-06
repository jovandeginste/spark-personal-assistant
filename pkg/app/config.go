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
	UserData UserData     `mapstructure:"user_data"`
	Context  string       `mapstructure:"context"`
	LLM      *ai.AIConfig `mapstructure:"llm"`

	AssistantFileCLI string                     `mapstructure:"-"`
	Assistant        ai.AssistantConfig         `mapstructure:"assistant"`
	Matrix           map[string]MatrixConfig    `mapstructure:"matrix"`
	Webserver        map[string]WebserverConfig `mapstructure:"webserver"`
	Mail             map[string]MailConfig      `mapstructure:"mail"`
	MCPServers       map[string]MCPServerConfig `mapstructure:"mcp_servers"`
	Prompts          map[string]string          `mapstructure:"prompts"`
}

type IMAPConfig struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}

type SMTPConfig struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}

type MailConfig struct {
	Enabled   *bool      `mapstructure:"enabled"`
	IMAP      IMAPConfig `mapstructure:"imap"`
	SMTP      SMTPConfig `mapstructure:"smtp"`
	To        string     `mapstructure:"to"`
	Allowlist []string   `mapstructure:"allowlist"`
	Username  string     `mapstructure:"username"`
	Password  string     `mapstructure:"password"`
	Folder    string     `mapstructure:"folder"`
	UseTLS    bool       `mapstructure:"use_tls"`
}

func (c MailConfig) IsEnabled() bool {
	if c.Enabled == nil {
		return true // default enabled
	}
	return *c.Enabled
}

type MCPServerConfig struct {
	Command   string   `mapstructure:"command"`
	Args      []string `mapstructure:"args"`
	Env       []string `mapstructure:"env"`
	URL       string   `mapstructure:"url"`
	Transport string   `mapstructure:"transport"`
	Token     string   `mapstructure:"token"`
}

type MatrixConfig struct {
	Enabled       *bool    `mapstructure:"enabled"`
	Homeserver    string   `mapstructure:"homeserver"`
	Username      string   `mapstructure:"username"`
	Password      string   `mapstructure:"password"`
	RoomID        string   `mapstructure:"room_id"`
	CryptoStore   string   `mapstructure:"database"`
	Users         []string `mapstructure:"users"`
	ThreadedTools bool     `mapstructure:"threaded_tools"`
}

func (c MatrixConfig) IsEnabled() bool {
	if c.Enabled == nil {
		return true // default enabled
	}
	return *c.Enabled
}

type WebserverConfig struct {
	Enabled *bool  `mapstructure:"enabled"`
	Bind    string `mapstructure:"bind"`
}

func (c WebserverConfig) IsEnabled() bool {
	if c.Enabled == nil {
		return true // default enabled
	}
	return *c.Enabled
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
		a.setMatrixCryptoStorePath,
		a.configureAssistant,
	} {
		if err := f(); err != nil {
			return err
		}
	}

	a.SetDefaults()

	return nil
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

func (a *App) setMatrixCryptoStorePath() error {
	absPath, err := filepath.Abs(a.ConfigFile)
	if err != nil {
		return err
	}
	dirname := filepath.Dir(absPath)

	for k, mc := range a.Config.Matrix {
		if mc.CryptoStore != "" && !strings.HasPrefix(mc.CryptoStore, "/") {
			mc.CryptoStore = filepath.Clean(filepath.Join(dirname, mc.CryptoStore))
			a.Config.Matrix[k] = mc
		}
	}

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
	a.Config.Assistant.File = filepath.Clean(filepath.Join(dirname, a.Config.Assistant.File))

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
	for k, wb := range a.Config.Webserver {
		if wb.Bind == "" {
			wb.Bind = ":8080"
			a.Config.Webserver[k] = wb
		}
	}

	if a.Config.Assistant.Language == "" {
		a.Config.Assistant.Language = "English"
	}

	for k, mailCfg := range a.Config.Mail {
		if mailCfg.IMAP.Port == 0 {
			if mailCfg.UseTLS {
				mailCfg.IMAP.Port = 993
			} else {
				mailCfg.IMAP.Port = 143
			}
			a.Config.Mail[k] = mailCfg
		}
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
