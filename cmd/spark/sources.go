package main

import (
	"os"

	"github.com/jovandeginste/spark-personal-assistant/pkg/structs"
	"github.com/spf13/cobra"
)

func (c *cli) sourcesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sources",
		Short: "Manage sources",
		Args:  cobra.NoArgs,
	}

	cmd.AddCommand(c.listSourcesCmd())
	cmd.AddCommand(c.updateSourceCmd())
	cmd.AddCommand(c.updateSourcesCmd())
	cmd.AddCommand(c.addSourceCmd())
	cmd.AddCommand(c.setSourceCmd())
	cmd.AddCommand(c.showSourceCmd())
	cmd.AddCommand(c.deleteSourceCmd())
	cmd.AddCommand(c.switchSourceCmd())

	return cmd
}

func (c *cli) updateSourcesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update-all",
		Short: "Update the entries of all source",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.app.UpdateSources()
		},
	}

	return cmd
}

func (c *cli) updateSourceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update name",
		Short: "Update the entries of a source",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, err := c.app.FindSourceByName(args[0])
			if err != nil {
				return err
			}

			c.app.Logger().Info("Updating entries", "source", src.Name)

			entries, err := src.RetrieveEntries()
			if err != nil {
				return err
			}

			if len(entries) == 0 {
				c.app.Logger().Info("No entries found", "source", src.Name)
				return nil
			}

			c.app.Logger().Info("Entries retrieved", "source", src.Name, "count", len(entries))

			c.app.FetchExistingEntries(src.ID, entries)

			if err := c.app.ReplaceSourceEntries(src, entries); err != nil {
				return err
			}

			c.app.Logger().Info("Entries updated", "source", src.Name, "count", len(entries))

			return nil
		},
	}

	return cmd
}

func (c *cli) showSourceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show name",
		Short: "Show a source",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, err := c.app.FindSourceByName(args[0])
			if err != nil {
				return err
			}

			src.PrintTo(os.Stdout)

			return nil
		},
	}

	return cmd
}

func (c *cli) setSourceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set name key value",
		Short: "Set metadata of a source",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[1]
			value := args[2]

			src, err := c.app.FindSourceByName(args[0])
			if err != nil {
				return err
			}

			src.SetMetadataIfNotEmpty(key, value)

			if err := c.app.UpdateSource(src); err != nil {
				return err
			}

			src.PrintTo(os.Stdout)

			return nil
		},
	}

	return cmd
}

func (c *cli) switchSourceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "switch name",
		Short: "Change the type of a source",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			srcName := args[0]
			srcType := args[1]

			src, err := c.app.FindSourceByName(srcName)
			if err != nil {
				return err
			}

			src.Type, err = structs.NewSourceType(srcType)
			if err != nil {
				return err
			}

			if err := c.app.UpdateSource(src); err != nil {
				return err
			}

			src.PrintTo(os.Stdout)

			return nil
		},
	}

	return cmd
}

func (c *cli) addSourceCmd() *cobra.Command {
	var s structs.Source

	cmd := &cobra.Command{
		Use:   "add name",
		Short: "Add a source",
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
	cmd.Flags().VarP(&s.Type, "type", "t", "Type of the source")

	return cmd
}

func (c *cli) listSourcesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List sources",
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
