package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"example.com/notes/internal/output"
	"example.com/notes/internal/store"
)

func newAdd() *cobra.Command {
	var tags []string
	c := &cobra.Command{
		Use:   "add <text>",
		Short: "Add a note",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return usageErrorf("add takes exactly one argument: the note text")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			f, err := formatOf(cmd)
			if err != nil {
				return err
			}
			text := strings.TrimSpace(args[0])
			if text == "" {
				return fmt.Errorf("note text must not be empty")
			}
			var clean []string
			for _, t := range tags {
				if t = strings.TrimSpace(t); t != "" {
					clean = append(clean, t)
				}
			}
			s, err := store.Open()
			if err != nil {
				return err
			}
			n := s.Add(text, clean)
			if err := s.Save(); err != nil {
				return err
			}
			if f == output.JSON {
				return output.WriteJSON(cmd.OutOrStdout(), map[string]int{"id": n.ID})
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "added #%d\n", n.ID)
			return err
		},
	}
	c.Flags().StringArrayVar(&tags, "tag", nil, "tag for the note (repeatable)")
	return c
}
