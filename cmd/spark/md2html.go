package main

import (
	"fmt"
	"os"

	"github.com/jovandeginste/spark-personal-assistant/pkg/helpers/md"
	"github.com/spf13/cobra"
)

func (c *cli) md2htmlCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "md2html [file.md]",
		Short:   "Convert markdown to HTML",
		Example: "spark md2html ./md/summary-full.md",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var (
				html string
				err  error
			)

			if len(args) == 0 {
				html, err = md.MDToHTML(os.Stdin)
			} else {
				html, err = md.MDFileToHTML(args[0])
			}

			if err != nil {
				return err
			}

			fmt.Print(html)
			return nil
		},
	}

	return cmd
}
