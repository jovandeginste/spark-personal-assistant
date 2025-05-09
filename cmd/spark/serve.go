package main

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/chzyer/readline"
	"github.com/jovandeginste/spark-personal-assistant/pkg/ai"
	"github.com/jovandeginste/spark-personal-assistant/pkg/app"
	"github.com/jovandeginste/spark-personal-assistant/pkg/data"
	"github.com/spf13/cobra"
)

type ChatHistory struct {
	Role    string
	Content string
}
type AIData struct {
	ExtraContext     []string
	ChatHistory      []ChatHistory `json:",omitempty"`
	EmployerQuestion []string      `json:",omitempty"`
	UserData         app.UserData
	Entries          data.Entries
}

func (c *cli) printCmd() *cobra.Command {
	var (
		ef           app.EntryFilter
		format       string
		customPrompt []string
	)

	cmd := &cobra.Command{
		Use:     "print",
		Short:   "Print Spark summary",
		Example: "spark print",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := c.app.Initialize(); err != nil {
				return err
			}

			aiData, err := c.buildData(ef)
			if err != nil {
				return err
			}

			aiData.EmployerQuestion = customPrompt

			p, err := ai.PromptFor(format)
			if err != nil {
				return err
			}

			aiClient, err := ai.NewClient(c.app.Config.LLM, c.app.Config.Assistant)
			if err != nil {
				return err
			}

			c.app.Logger().Info(
				"Generating summary for entries...",
				"type", c.app.Config.LLM.Type,
				"model", c.app.Config.LLM.Model,
				"name", c.app.Config.Assistant.Name,
			)

			md, err := aiClient.GeneratePrompt(context.Background(), p, aiData)
			if err != nil {
				return err
			}

			fmt.Println(md)

			return nil
		},
	}

	cmd.Flags().StringSliceVarP(&customPrompt, "prompt", "p", nil, "extra custom prompt")
	cmd.Flags().StringVar(&c.app.ConfigFile, "config", "./spark.yaml", "config file")
	cmd.Flags().StringVarP(&format, "format", "f", "full", "Format to use")
	cmd.Flags().UintVarP(&ef.DaysBack, "days-back", "b", 3, "Number of days in the past to include")
	cmd.Flags().UintVarP(&ef.DaysAhead, "days-ahead", "a", 7, "Number of days in the future to include")

	return cmd
}

func (c *cli) chatCmd() *cobra.Command {
	var ef app.EntryFilter

	cmd := &cobra.Command{
		Use:     "chat",
		Short:   "Chat with Spark",
		Example: "spark chat",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := c.app.Initialize(); err != nil {
				return err
			}

			aiData, err := c.buildData(ef)
			if err != nil {
				return err
			}

			p, err := ai.PromptFor("custom")
			if err != nil {
				return err
			}

			aiClient, err := ai.NewClient(c.app.Config.LLM, c.app.Config.Assistant)
			if err != nil {
				return err
			}

			c.app.Logger().Info(
				"Generating summary for entries...",
				"type", c.app.Config.LLM.Type,
				"model", c.app.Config.LLM.Model,
				"name", c.app.Config.Assistant.Name,
			)

			rl, err := readline.New("> ")
			if err != nil {
				return err
			}

			defer rl.Close() // Ensure readline resources are cleaned up when the program exits

			fmt.Println("Enter your question. Type /quit to exit or press Ctrl+D.")

		input:
			for {
				fmt.Print("> ")

				input, err := rl.Readline()
				switch err {
				case nil:
				case io.EOF: // Exit the loop on Ctrl+D (EOF)
					fmt.Println("\nGoodbye!")
					break input
				case readline.ErrInterrupt:
					continue // Clear the current line and continue to the next prompt
				default:
					fmt.Println("Error reading input:", err)
					continue
				}

				input = strings.TrimSpace(input)
				switch input {
				case "":
					continue
				case "/quit":
					fmt.Println("Goodbye!")
					break input
				}

				aiData.EmployerQuestion = []string{input}

				c.app.Logger().Info("Parsing your question...")

				md, err := aiClient.GeneratePrompt(context.Background(), p, aiData)
				if err != nil {
					return err
				}

				fmt.Println(md)

				aiData.ChatHistory = append(
					aiData.ChatHistory,
					ChatHistory{Role: "user", Content: input},
					ChatHistory{Role: "assistant", Content: md},
				)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&c.app.ConfigFile, "config", "./spark.yaml", "config file")
	cmd.Flags().UintVarP(&ef.DaysBack, "days-back", "b", 3, "Number of days in the past to include")
	cmd.Flags().UintVarP(&ef.DaysAhead, "days-ahead", "a", 7, "Number of days in the future to include")

	return cmd
}

func (c *cli) buildData(ef app.EntryFilter) (*AIData, error) {
	entries, err := c.app.CurrentEntries(ef)
	if err != nil {
		return nil, err
	}

	aiData := &AIData{
		ExtraContext: c.app.Config.ExtraContext,
		UserData:     c.app.Config.UserData,
		Entries:      entries,
	}

	return aiData, nil
}
