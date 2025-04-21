package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/jovandeginste/spark-personal-assistant/pkg/data"
	"github.com/spf13/cobra"
)

func (c *cli) entriesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "entries",
		Short: "Manage entries",
		Args:  cobra.NoArgs,
	}

	cmd.AddCommand(c.listEntriesCmd())
	cmd.AddCommand(c.addEntryCmd())
	cmd.AddCommand(c.showEntryCmd())

	return cmd
}

func (c *cli) showEntryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show id",
		Short: "Show an entry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return err
			}

			e := &data.Entry{ID: id}

			if err := c.app.FindEntry(e); err != nil {
				return err
			}

			e.PrintTo(os.Stdout)

			return nil
		},
	}

	return cmd
}

func (c *cli) addEntryCmd() *cobra.Command {
	var (
		e data.Entry
		d string
		i string
		s string
	)

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add an entry",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			src, err := c.app.FindSourceByName(s)
			if err != nil {
				return err
			}

			e.Source = *src

			if err := e.SetDate(d); err != nil {
				return err
			}

			if err := e.SetImportance(i); err != nil {
				return err
			}

			if err := c.app.CreateEntry(e); err != nil {
				return err
			}

			fmt.Println("Entry added:")
			e.PrintTo(os.Stdout)

			return nil
		},
	}

	cmd.Flags().StringVarP(&e.Summary, "title", "t", "", "Title of the entry")
	cmd.Flags().StringVarP(&i, "importance", "i", string(data.MEDIUM), "Importance of the entry")
	cmd.Flags().StringVarP(&d, "date", "d", "", "Date of the entry")
	cmd.Flags().StringVarP(&s, "source", "s", "manual", "Source of the entry")

	return cmd
}

func (c *cli) listEntriesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List entries",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			entries, err := c.app.Entries()
			if err != nil {
				return err
			}

			entries.PrintTo(os.Stdout)

			return nil
		},
	}

	return cmd
}
