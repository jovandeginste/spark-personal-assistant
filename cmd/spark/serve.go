package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/chzyer/readline"
	"github.com/jovandeginste/spark-personal-assistant/pkg/ai"

	// "github.com/jovandeginste/spark-personal-assistant/pkg/app"
	"github.com/spf13/cobra"
	"github.com/yarlson/pin"
)

func (c *cli) printCmd() *cobra.Command {
	var (
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

			aiData, err := c.app.BuildData()
			if err != nil {
				return err
			}

			aiData.EmployerQuestion = customPrompt

			p, err := ai.PromptFor(format)
			if err != nil {
				return err
			}

			aiClient, err := ai.NewClient(c.app.Config.LLM, &c.app.Config.Assistant, c.app.Logger())
			if err != nil {
				return err
			}

			c.app.Logger().Info(
				"Generating summary for entries...",
				"type", c.app.Config.LLM.Type,
				"model", c.app.Config.LLM.Model,
				"name", c.app.Config.Assistant.Name,
				"language", c.app.Config.Assistant.Language,
			)

			spinner := pin.New("Thinking...",
				pin.WithSpinnerColor(pin.ColorCyan),
				pin.WithTextColor(pin.ColorYellow),
				pin.WithWriter(os.Stderr),
			)

			cancel := spinner.Start(context.Background())
			defer cancel()

			md, err := aiClient.GeneratePrompt(context.Background(), p, aiData)
			if err != nil {
				return err
			}

			spinner.Stop("Ready!")
			fmt.Println(md)

			return nil
		},
	}

	cmd.Flags().StringSliceVarP(&customPrompt, "prompt", "p", nil, "extra custom prompt")
	cmd.Flags().StringVar(&c.app.ConfigFile, "config", "./spark.yaml", "config file")
	cmd.Flags().StringVar(&c.app.Config.AssistantFileCLI, "persona", "", "persona")
	cmd.Flags().StringVarP(&format, "format", "f", "full", "Format to use")

	return cmd
}

func (c *cli) chatCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "chat",
		Short:   "Chat with Spark",
		Example: "spark chat",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := c.app.Initialize(); err != nil {
				return err
			}

			aiData, err := c.app.BuildData()
			if err != nil {
				return err
			}

			p, err := ai.PromptFor("custom")
			if err != nil {
				return err
			}

			aiClient, err := ai.NewClient(c.app.Config.LLM, &c.app.Config.Assistant, c.app.Logger())
			if err != nil {
				return err
			}

			c.app.Logger().Info(
				"Generating summary for entries...",
				"type", c.app.Config.LLM.Type,
				"model", c.app.Config.LLM.Model,
				"name", c.app.Config.Assistant.Name,
				"language", c.app.Config.Assistant.Language,
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

				spinner := pin.New("Thinking...",
					pin.WithSpinnerColor(pin.ColorCyan),
					pin.WithTextColor(pin.ColorYellow),
					pin.WithWriter(os.Stderr),
				)

				cancel := spinner.Start(context.Background())
				defer cancel()

				md, err := aiClient.GeneratePrompt(context.Background(), p, aiData)
				if err != nil {
					return err
				}

				spinner.Stop("Ready!")
				fmt.Println(md)
				aiData.AddChatHistory("cli", "user", input)
				aiData.AddChatHistory("cli", "assistant", md)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&c.app.ConfigFile, "config", "./spark.yaml", "config file")
	cmd.Flags().StringVar(&c.app.Config.AssistantFileCLI, "persona", "", "persona")

	return cmd
}
