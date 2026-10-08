package cmd

import (
	"github.com/spf13/cobra"

	"github.com/marcellovictorino/pill/internal/output"
)

func newLsCmd(a *App) *cobra.Command {
	return &cobra.Command{
		Use:   "ls",
		Short: "List local models, their state and size",
		Long: `List models known to pill on this machine.

state is pulled (a downloaded GGUF with no entry), unverified, passed, failed,
or missing (declared in models.toml but the file is not here).`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := a.load(); err != nil {
				return err
			}
			snap, err := a.reg.Load()
			if err != nil {
				return output.Fail(nil, "%v", err)
			}
			rows := a.reg.Rows(snap)
			notes := a.olderBuildNotes(snap)
			table := make([]output.Obj, 0, len(rows))
			for _, r := range rows {
				state, size := r.State, humanBytes(r.Size)
				if r.Missing {
					state, size = "missing", "n/a"
				}
				def := "no"
				if r.Default {
					def = "yes"
				}
				row := output.Obj{}.Set("name", r.Name).Set("state", state).Set("size", size).Set("default", def)
				if len(notes) > 0 { // the column exists only when something needs flagging
					row = row.Set("note", orDash(notes[r.Name]))
				}
				table = append(table, row)
			}
			var help []string
			if len(rows) == 0 {
				help = append(help, "see what you can download: pill catalog", "download one: pill pull gemma4-26b")
			}
			a.printer.Emit(output.Obj{}.Set("models", table).Set("help", help))
			return nil
		},
	}
}
