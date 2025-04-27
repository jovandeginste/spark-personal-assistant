package main

import (
	"fmt"
	"io"
	"os"

	"github.com/jovandeginste/spark-personal-assistant/pkg/markdown"
	"github.com/spf13/cobra"
)

func (c *cli) md2htmlCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "md2html [file.md]",
		Short:   "Convert markdown to HTML",
		Example: "spark md2html ./md/summary-full.md",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return mdToHTML(os.Stdin)
			}

			file, err := os.Open(args[0])
			if err != nil {
				return err
			}

			return mdToHTML(file)
		},
	}

	return cmd
}

func mdToHTML(file io.Reader) error {
	md, err := io.ReadAll(file)
	if err != nil {
		return err
	}

	html, err := markdown.GenerateHTML(md)
	if err != nil {
		return err
	}

	fmt.Println(string(html))

	return nil
}
