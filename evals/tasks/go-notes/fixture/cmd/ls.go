package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"example.com/notes/internal/output"
	"example.com/notes/internal/store"
)

type noteJSON struct {
	ID      int      `json:"id"`
	Text    string   `json:"text"`
	Tags    []string `json:"tags"`
	Created string   `json:"created"`
}

func newLs() *cobra.Command {
	var tag string
	c := &cobra.Command{
		Use:   "ls",
		Short: "List notes, optionally filtered by tag",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 0 {
				return usageErrorf("ls takes no arguments")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			f, err := formatOf(cmd)
			if err != nil {
				return err
			}
			s, err := store.Open()
			if err != nil {
				return err
			}
			var notes []store.Note
			for _, n := range s.Notes() {
				if tag == "" || n.HasTag(tag) {
					notes = append(notes, n)
				}
			}
			out := cmd.OutOrStdout()
			if f == output.JSON {
				list := []noteJSON{}
				for _, n := range notes {
					tags := n.Tags
					if tags == nil {
						tags = []string{}
					}
					list = append(list, noteJSON{n.ID, n.Text, tags, n.Created.Format(time.RFC3339)})
				}
				return output.WriteJSON(out, map[string]any{"notes": list})
			}
			if len(notes) == 0 {
				_, err = fmt.Fprintln(out, "no notes")
				return err
			}
			for _, n := range notes {
				line := fmt.Sprintf("#%d %s", n.ID, n.Text)
				if len(n.Tags) > 0 {
					line += " [" + strings.Join(n.Tags, ",") + "]"
				}
				fmt.Fprintln(out, line)
			}
			return nil
		},
	}
	c.Flags().StringVar(&tag, "tag", "", "only notes with this tag (case-insensitive)")
	return c
}
