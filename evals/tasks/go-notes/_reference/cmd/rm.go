package cmd

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"example.com/notes/internal/output"
	"example.com/notes/internal/store"
)

type rmJSON struct {
	Removed []int `json:"removed"`
	Missing []int `json:"missing"`
}

func newRm() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <id>...",
		Short: "Remove notes by id (a missing id is reported, not an error)",
		RunE: func(cmd *cobra.Command, args []string) error {
			f, err := formatOf(cmd)
			if err != nil {
				return err
			}
			if len(args) == 0 {
				return usageErrorf("rm takes one or more note ids")
			}
			// Validate every id before touching the store.
			ids := make([]int, 0, len(args))
			for _, a := range args {
				id, err := strconv.Atoi(a)
				if err != nil || id < 1 {
					return usageErrorf("invalid note id %q: ids are positive integers", a)
				}
				ids = append(ids, id)
			}
			s, err := store.Open()
			if err != nil {
				return err
			}
			removed, missing := []int{}, []int{}
			var lines []string
			for _, id := range ids {
				if s.Remove(id) {
					removed = append(removed, id)
					lines = append(lines, fmt.Sprintf("removed #%d", id))
				} else {
					missing = append(missing, id)
					lines = append(lines, fmt.Sprintf("not found #%d", id))
				}
			}
			if len(removed) > 0 {
				if err := s.Save(); err != nil {
					return err
				}
			}
			out := cmd.OutOrStdout()
			if f == output.JSON {
				return output.WriteJSON(out, rmJSON{removed, missing})
			}
			for _, l := range lines {
				fmt.Fprintln(out, l)
			}
			return nil
		},
	}
}
