package main

import (
	"encoding/json"
	"os"

	"github.com/jovandeginste/spark-personal-assistant/pkg/data"
	"github.com/spf13/cobra"
)

func (c *cli) sourcesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sources",
		Short: "Manage sources",
		Args:  cobra.NoArgs,
	}

	cmd.AddCommand(c.listSourcesCmd())
	cmd.AddCommand(c.addSourceCmd())
	cmd.AddCommand(c.deleteSourceCmd())
	cmd.AddCommand(c.replaceEntriesSourceCmd())

	return cmd
}

func (c *cli) addSourceCmd() *cobra.Command {
	var s data.Source

	cmd := &cobra.Command{
		Use:   "add name",
		Short: "Add an entry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s.Name = args[0]

			if err := c.app.CreateSource(&s); err != nil {
				return err
			}

			c.app.Logger().Info("Source added")
			s.PrintTo(os.Stdout)

			return nil
		},
	}

	cmd.Flags().StringVarP(&s.Description, "description", "d", "", "Description of the source")

	return cmd
}

func (c *cli) listSourcesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List entries",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			sources, err := c.app.Sources()
			if err != nil {
				return err
			}

			sources.PrintTo(os.Stdout)

			return nil
		},
	}

	return cmd
}

func (c *cli) replaceEntriesSourceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "replace-entries",
		Short: "Replace all entries for source",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, err := c.app.FindSourceByName(args[0])
			if err != nil {
				return err
			}

			entriesFile := args[1]
			ef, err := os.ReadFile(entriesFile)
			if err != nil {
				return err
			}

			var entries data.Entries
			if err := json.Unmarshal(ef, &entries); err != nil {
				return err
			}

			c.app.FetchExistingEntries(entries)

			if err := c.app.ReplaceSourceEntries(src, entries); err != nil {
				return err
			}

			return nil
		},
	}

	return cmd
}

func (c *cli) deleteSourceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete id",
		Short: "Delete a source",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, err := c.app.FindSourceByName(args[0])
			if err != nil {
				return err
			}

			if err := c.app.DeleteSource(src); err != nil {
				return err
			}

			c.app.Logger().Info("Source deleted", "name", src.Name, "id", src.ID)

			return nil
		},
	}

	return cmd
}
