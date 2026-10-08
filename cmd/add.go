package cmd

import (
	"context"
	"time"

	"github.com/spf13/cobra"

	"github.com/marcellovictorino/pill/internal/config"
	"github.com/marcellovictorino/pill/internal/output"
	"github.com/marcellovictorino/pill/internal/registry"
)

func newAddCmd(a *App) *cobra.Command {
	var unverified bool
	var ctxSize int
	c := &cobra.Command{
		Use:   "add <name> --unverified",
		Short: "Register a local GGUF with the router and Pi, without benchmarking it",
		Long: `Register a model whose GGUF is already in the models directory: it is added to
models.toml, the generated models.ini, and Pi's "pill" provider.

<name> is a catalog name (gemma4-26b, gemma4-26b-iq4xs-64k), a .gguf file name,
or a Hugging Face reference (hf.co/owner/repo:Q4_K_M). The model is marked
"unverified" everywhere it is shown, because it has not passed 'pill bench run'
on this machine. --unverified is required so that choice is always explicit.`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: a.completeModels(modelSources{catalog: true, registered: true, files: true}),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !unverified {
				return output.Usage([]string{
					"to accept the risk: pill add " + args[0] + " --unverified",
					"to prove it first: pill bench run " + args[0],
				}, "add registers a model that has not passed the benchmark; pass --unverified to confirm")
			}
			if err := a.load(); err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 3*time.Minute)
			defer cancel()
			return runAdd(ctx, a, args[0], ctxSize)
		},
	}
	c.Flags().BoolVar(&unverified, "unverified", false, "confirm registering a model that has not been benchmarked")
	c.Flags().IntVar(&ctxSize, "ctx", 0, "context size in tokens (default: the catalog value, else 32768)")
	return c
}

func runAdd(ctx context.Context, a *App, arg string, ctxSize int) error {
	var m config.Model
	changed := false
	// Read, change and write under the state lock: a benchmark or another
	// add running at the same time must not have its changes overwritten.
	snap, applied, err := a.reg.UpdateApply(ctx, func(s *snapshot) error {
		var err error
		m, err = a.reg.Resolve(s, arg, ctxSize)
		if err != nil {
			return output.Fail([]string{"see the built-in models: pill catalog", "or name a GGUF in " + a.settings.ModelsDir}, "%v", err)
		}
		if m.File == "" || a.reg.FileSize(m) == 0 {
			file := m.File
			if file == "" {
				file = "a GGUF for " + arg
			}
			return output.Fail([]string{
				"download it: pill pull " + arg,
				"or copy the file into " + a.settings.ModelsDir,
			}, "%s is not in %s", file, a.settings.ModelsDir)
		}

		before, existed := s.Models.Find(m.Name)
		changed = !existed || !sameModel(*before, m)
		s.Models.Upsert(m)

		st := s.Results.Get(m.Name)
		switch {
		case st == nil:
			st = s.Results.Ensure(m.Name)
			changed = true
		case !st.InPi:
			changed = true
		}
		if a.reg.State(s, m) == config.StateFailed {
			st.State = config.StateUnverified // the explicit override of a failed benchmark
			changed = true
		}
		st.InPi = true
		if !changed {
			return registry.ErrNoChange // models.toml may live in dotfiles: do not rewrite it for nothing
		}
		return nil
	})
	if err != nil {
		return wrapState(err)
	}
	if a.rt.Healthy(ctx) { // refresh a running router; never start one just for add
		if _, err := a.ensureRouter(ctx, applied); err != nil {
			return err
		}
	}

	status := "registered"
	if !changed && !applied.IniChanged && !applied.PiChanged {
		status = "already registered"
	}
	help := []string{}
	if a.settings.Default != m.Name {
		help = append(help, "make it the default: pill default "+m.Name)
	}
	a.printer.Emit(output.Obj{}.
		Set("model", m.Name).
		Set("status", status).
		Set("state", a.reg.State(snap, m)).
		Set("ctx", m.Ctx).
		Set("pi", "pill/"+m.Name).
		Set("help", help))
	return nil
}

func sameModel(a, b config.Model) bool {
	if a.Name != b.Name || a.File != b.File || a.Ctx != b.Ctx || a.Repo != b.Repo ||
		a.Reasoning != b.Reasoning || a.MaxTokens != b.MaxTokens || a.Quant != b.Quant || len(a.Preset) != len(b.Preset) {
		return false
	}
	for k, v := range a.Preset {
		if w, ok := b.Preset[k]; !ok || w != v {
			return false
		}
	}
	return true
}
