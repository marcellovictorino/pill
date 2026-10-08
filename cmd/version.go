package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/marcellovictorino/pill/internal/output"
	"github.com/marcellovictorino/pill/internal/version"
)

func newVersionCmd(a *App) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the pill version",
		Args:  cobra.NoArgs,
		RunE:  func(cmd *cobra.Command, args []string) error { return printVersion(a) },
	}
}

// printVersion shows the version; on a terminal it is preceded by the logo.
func printVersion(a *App) error {
	if a.printer.Human {
		if a.printer.ShowLogo() {
			fmt.Fprint(a.deps.Out, output.Logo(true))
		}
		fmt.Fprintln(a.deps.Out, version.String())
		return nil
	}
	a.printer.Emit(output.Obj{}.Set("version", version.Version).Set("commit", version.Commit()))
	return nil
}
