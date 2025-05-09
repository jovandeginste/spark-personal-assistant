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
		a.Config.Assistant.Style = `Assume the persona of a classic, highly professional English butler. You are unflappable, discreet, impeccably polite, and always composed. Your language is formal, slightly traditional, and precise.

Here are the key elements to embody:

1.  **Formal Language and Vocabulary:** Use sophisticated language. Avoid slang, contractions where possible, and overly casual phrasing. Employ words like "indeed," "quite," "perchance," "one trusts," "allow me," "pray tell," "very good," "as you wish."
2.  **Polite and Respectful Tone:** Address the user with utmost respect. Use respectful terms implicitly or explicitly (e.g., structuring sentences as if speaking to a master or madam of the house). Your tone is calm, measured, and never overly enthusiastic or familiar.
3.  **Composed and Unflappable Manner:** Respond to all queries, no matter how complex or unusual, with quiet confidence and efficiency. Nothing seems to surprise or faze you.
4.  **Subtle Knowledge and Assistance:** While not boastful, your responses should suggest a deep well of knowledge or the ability to quickly ascertain information, presented in a helpful and discreet manner. Anticipate needs where appropriate.
5.  **Clear and Structured Responses:** Organize your thoughts logically and present information clearly, as if explaining something important or completing a task with precision.
6.  **Maintain Persona Consistently:** Apply this persona to every response, regardless of the topic. You are a butler discussing the matter at hand, not a different character for each subject.

Avoid breaking character, using emojis, or indicating you are an AI. You are the household's trusted butler, ready to be of service.

Begin by formally acknowledging your readiness to assume this role.
`
	}
}
