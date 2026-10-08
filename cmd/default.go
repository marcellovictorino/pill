package cmd

import (
	"github.com/spf13/cobra"

	"github.com/marcellovictorino/pill/internal/config"
	"github.com/marcellovictorino/pill/internal/output"
)

func newDefaultCmd(a *App) *cobra.Command {
	return &cobra.Command{
		Use:   "default [name]",
		Short: "Set (or show) the model that `pill` opens Pi on",
		Long: `Set the default model. It must be registered, present on disk, and either
passed or unverified (a failed model cannot be the default).

Without an argument, shows the current default.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := a.load(); err != nil {
				return err
			}
			if len(args) == 0 {
				a.printer.Emit(output.Obj{}.Set("default", orDash(a.settings.Default)))
				return nil
			}
			return runDefault(a, args[0])
		},
	}
}

func runDefault(a *App, arg string) error {
	snap, err := a.reg.Load()
	if err != nil {
		return output.Fail(nil, "%v", err)
	}
	name := arg
	if _, ok := snap.Models.Find(name); !ok {
		// Allow the short catalog name (gemma4-26b) if its variant is registered.
		if e, ok := a.reg.Catalog.Resolve(arg); ok {
			name = e.Name
		}
	}
	m, ok := snap.Models.Find(name)
	if !ok {
		return output.Fail([]string{"register it first: pill add " + arg + " --unverified", "list models: pill ls"}, "%s is not registered", arg)
	}
	state := a.reg.State(snap, *m)
	st := snap.Results.Get(m.Name)
	switch {
	case a.reg.FileSize(*m) == 0:
		return output.Fail([]string{"download it: pill pull " + m.Name}, "the file for %s (%s) is missing", m.Name, m.File)
	case state == config.StateFailed:
		return output.Fail([]string{"override the result: pill add " + m.Name + " --unverified"}, "%s failed its benchmark and cannot be the default", m.Name)
	case st == nil || !st.InPi:
		return output.Fail([]string{"pill add " + m.Name + " --unverified"}, "%s is not registered with Pi", m.Name)
	}
	status := "set"
	if a.settings.Default == m.Name {
		status = "unchanged"
	} else if err := config.SetDefault(a.paths, m.Name); err != nil {
		return output.Fail(nil, "%v", err)
	}
	a.printer.Emit(output.Obj{}.Set("default", m.Name).Set("status", status).Set("state", state).
		Set("help", []string{"open Pi on it: pill"}))
	return nil
}
