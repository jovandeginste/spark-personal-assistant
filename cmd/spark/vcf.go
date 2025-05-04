package main

import (
	"github.com/jovandeginste/spark-personal-assistant/pkg/helpers/vcf"
	"github.com/spf13/cobra"
)

func (c *cli) vcfCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "vcf2entry source file.vcf",
		Short:   "Convert vcf to Spark birthday entries",
		Example: "spark vcf2entry birthdays ./contacts.vcf",
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, err := c.app.FindSourceByName(args[0])
			if err != nil {
				return err
			}

			file := args[1]

			entries, err := vcf.BuildEntriesFromFile(file)

			c.app.FetchExistingEntries(entries)

			return c.app.ReplaceSourceEntries(src, entries)
		},
	}

	return cmd
}
