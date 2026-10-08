package cmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/marcellovictorino/pill/internal/bench"
	"github.com/marcellovictorino/pill/internal/config"
	"github.com/marcellovictorino/pill/internal/output"
)

func newSyncCmd(a *App) *cobra.Command {
	var doBench bool
	th := bench.DefaultThresholds()
	c := &cobra.Command{
		Use:   "sync",
		Short: "Download the models declared in models.toml that are missing here",
		Long: `Bring a new machine up to date with ~/.config/pill/models.toml (the file you
keep in dotfiles). Every declared model whose GGUF is missing is downloaded and
registered as unverified, so it is usable at once but still labelled as not
proven on this Mac.

With --bench, every declared model that is still unverified is then
benchmarked (see 'pill bench run'); this takes about 25 minutes each.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := a.load(); err != nil {
				return err
			}
			return runSync(cmd.Context(), a, doBench, th)
		},
	}
	c.Flags().BoolVar(&doBench, "bench", false, "benchmark the models that are still unverified afterwards")
	return c
}

func runSync(ctx context.Context, a *App, doBench bool, th config.Thresholds) error {
	snap, err := a.reg.Load()
	if err != nil {
		return output.Fail(nil, "%v", err)
	}
	if len(snap.Models.Models) == 0 {
		a.printer.Emit(output.Obj{}.Set("declared", 0).Set("pulled", []string{}).
			Set("help", []string{"nothing is declared in " + a.paths.ModelsToml() + "; add one: pill add <name> --unverified"}))
		return nil
	}

	var rows []output.Obj
	var failed []string
	pulled := 0
	for _, m := range append([]config.Model(nil), snap.Models.Models...) {
		if a.reg.FileSize(m) > 0 {
			continue
		}
		if m.Repo == "" {
			rows = append(rows, output.Obj{}.Set("model", m.Name).Set("status", "skipped: no download source").Set("size", "n/a"))
			failed = append(failed, m.Name)
			continue
		}
		file, status, _, err := a.pullModel(ctx, snap, m)
		if err != nil {
			return err
		}
		m.File = file.Name()
		snap.Models.Upsert(m)
		st := snap.Results.Ensure(m.Name)
		st.InPi = true // usable at once, labelled unverified
		if a.reg.State(snap, m) == config.StateFailed {
			st.State = config.StateUnverified
		}
		pulled++
		rows = append(rows, output.Obj{}.Set("model", m.Name).Set("status", status).Set("size", humanBytes(file.Size)))
	}
	if err := a.reg.Save(snap); err != nil {
		return output.Fail(nil, "%v", err)
	}
	applied, err := a.reg.Apply(snap)
	if err != nil {
		return output.Fail(nil, "%v", err)
	}
	if a.rt.Healthy(ctx) { // refresh a running router; never start one for sync
		if _, err := a.ensureRouter(ctx, applied); err != nil {
			return err
		}
	}

	doc := output.Obj{}.Set("declared", len(snap.Models.Models)).Set("pulled", rows)
	var help []string
	if len(failed) > 0 {
		help = append(help, "copy the GGUF into "+a.settings.ModelsDir+" for: "+joinIDs(failed))
	}

	var benched []output.Obj
	allPassed := true
	if doBench {
		for _, m := range snap.Models.Models {
			if a.reg.FileSize(m) == 0 || a.reg.State(snap, m) != config.StateUnverified {
				continue
			}
			out, err := a.benchModel(ctx, m.Name, 0, 1, th)
			if err != nil {
				return err
			}
			if !out.Res.Passed {
				allPassed = false
			}
			benched = append(benched, output.Obj{}.Set("model", m.Name).Set("result", map[bool]string{true: "passed", false: "failed"}[out.Res.Passed]).
				Set("state", out.State).Set("dir", out.Dir))
		}
		doc = doc.Set("benchmarked", benched)
	}
	switch {
	case pulled == 0 && len(failed) == 0 && !doBench:
		help = append(help, "everything declared is already here; prove unverified models with: pill sync --bench")
	case !doBench:
		help = append(help, "prove them on this Mac: pill bench run <name> (or pill sync --bench)")
	}
	a.printer.Emit(doc.Set("help", help))
	if !allPassed {
		return output.ExitWith(output.ExitFailure)
	}
	if len(failed) > 0 {
		return output.ExitWith(output.ExitFailure)
	}
	return nil
}
