package main

import (
	"fmt"

	"github.com/jovandeginste/spark-personal-assistant/pkg/helpers/generic"
	"github.com/jovandeginste/spark-personal-assistant/pkg/helpers/md"
	"github.com/spf13/cobra"
)

func (c *cli) md2htmlCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "md2html [file.md]",
		Short:   "Convert markdown to HTML",
		Example: "spark md2html ./md/summary-full.md",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			f, err := generic.ReadResource(args[0])
			if err != nil {
				return err
			}

			html, err := md.MDToHTML(f)
			if err != nil {
				return err
			}

			fmt.Print(html)
			return nil
		},
	}

	return cmd
}
