package main

import (
	"fmt"

	"github.com/jovandeginste/spark-personal-assistant/pkg/helpers/generic"
	"github.com/jovandeginste/spark-personal-assistant/pkg/helpers/md"
	"github.com/spf13/cobra"
)

func (c *cli) md2textCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "md2text [file.md]",
		Short:   "Convert markdown to text",
		Example: "spark md2text ./md/summary-full.md",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			f, err := generic.ReadResource(args[0])
			if err != nil {
				return err
			}

			text, err := md.MDToText(f)
			if err != nil {
				return err
			}

			fmt.Print(text)
			return nil
		},
	}

	return cmd
}
