package cmd

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"example.com/notes/internal/output"
	"example.com/notes/internal/store"
)

type tagCount struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}

type statsJSON struct {
	Total  int        `json:"total"`
	Oldest *string    `json:"oldest"`
	Newest *string    `json:"newest"`
	Tags   []tagCount `json:"tags"`
}

func newStats() *cobra.Command {
	return &cobra.Command{
		Use:   "stats",
		Short: "Summarise the notes",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 0 {
				return usageErrorf("stats takes no arguments")
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
			notes := s.Notes()
			counts := map[string]int{}
			var oldest, newest time.Time
			for i, n := range notes {
				if i == 0 || n.Created.Before(oldest) {
					oldest = n.Created
				}
				if i == 0 || n.Created.After(newest) {
					newest = n.Created
				}
				seen := map[string]bool{}
				for _, t := range n.Tags {
					seen[strings.ToLower(t)] = true
				}
				for t := range seen {
					counts[t]++
				}
			}
			tags := []tagCount{}
			for t, c := range counts {
				tags = append(tags, tagCount{t, c})
			}
			sort.Slice(tags, func(i, j int) bool {
				if tags[i].Count != tags[j].Count {
					return tags[i].Count > tags[j].Count
				}
				return tags[i].Tag < tags[j].Tag
			})
			out := cmd.OutOrStdout()
			if f == output.JSON {
				st := statsJSON{Total: len(notes), Tags: tags}
				if len(notes) > 0 {
					o, n := oldest.Format(time.RFC3339), newest.Format(time.RFC3339)
					st.Oldest, st.Newest = &o, &n
				}
				return output.WriteJSON(out, st)
			}
			fmt.Fprintf(out, "total: %d\n", len(notes))
			if len(notes) == 0 {
				fmt.Fprintln(out, "oldest: -")
				fmt.Fprintln(out, "newest: -")
			} else {
				fmt.Fprintf(out, "oldest: %s\n", oldest.Format(time.RFC3339))
				fmt.Fprintf(out, "newest: %s\n", newest.Format(time.RFC3339))
			}
			if len(tags) == 0 {
				fmt.Fprintln(out, "tags: none")
				return nil
			}
			fmt.Fprintln(out, "tags:")
			for _, t := range tags {
				fmt.Fprintf(out, "  %s: %d\n", t.Tag, t.Count)
			}
			return nil
		},
	}
}
