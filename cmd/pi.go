package cmd

import (
	"context"
	"time"

	"github.com/spf13/cobra"

	"github.com/marcellovictorino/pill/internal/config"
	"github.com/marcellovictorino/pill/internal/output"
	"github.com/marcellovictorino/pill/internal/pi"
)

func newPiCmd(a *App) *cobra.Command {
	return &cobra.Command{
		Use:   "pi [pi args...]",
		Short: "Open Pi on the default local model (same as running pill)",
		Long: `Open the Pi coding agent on the default local model.

Everything after "pill pi" is passed to Pi unchanged. pill adds
"--model pill/<default> --thinking off" unless you pass your own --model or
--thinking.`,
		// Pass every argument through, including flags cobra would otherwise reject.
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return launchPi(cmd.Context(), a, args)
		},
	}
}

// launchPi makes sure the router is up, then replaces this process with Pi.
func launchPi(ctx context.Context, a *App, userArgs []string) error {
	if err := a.load(); err != nil {
		return err
	}
	name := a.settings.Default
	if name == "" {
		return output.Fail([]string{
			"register a model: pill add gemma4-26b-iq4xs-64k --unverified",
			"then choose it: pill default gemma4-26b-iq4xs-64k",
		}, "no default model set")
	}
	snap, err := a.reg.Load()
	if err != nil {
		return output.Fail(nil, "%v", err)
	}
	m, ok := snap.Models.Find(name)
	if !ok {
		return output.Fail([]string{"pill add " + name + " --unverified", "or pick another: pill default <name>"}, "default model %s is not in models.toml", name)
	}
	if a.reg.FileSize(*m) == 0 {
		return output.Fail([]string{"download it: pill pull " + name, "or copy the GGUF into " + a.settings.ModelsDir}, "the file for %s (%s) is missing", name, m.File)
	}
	if a.reg.State(snap, *m) == config.StateFailed {
		return output.Fail([]string{"pill default <other-model>", "pill add " + name + " --unverified to override"}, "default model %s failed its benchmark", name)
	}
	bin, err := pi.Binary()
	if err != nil {
		return output.Fail([]string{"install it: brew install pi-coding-agent"}, "%v", err)
	}

	applied, err := a.reg.Apply(snap)
	if err != nil {
		return output.Fail(nil, "%v", err)
	}
	startCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	a.printer.Progress("starting the router on port %d if needed...", a.settings.Port)
	st, err := a.ensureRouter(startCtx, applied)
	if err != nil {
		return err
	}
	if st.Stale {
		a.printer.Progress("note: the router predates your latest model changes; run `pill stop --all` to apply them")
	}
	return output.Fail(nil, "cannot start pi: %v", pi.Exec(bin, pi.Args(name, userArgs)))
}
