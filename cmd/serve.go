package cmd

import (
	"context"
	"time"

	"github.com/spf13/cobra"

	"github.com/marcellovictorino/pill/internal/output"
)

func newServeCmd(a *App) *cobra.Command {
	var foreground bool
	c := &cobra.Command{
		Use:   "serve",
		Short: "Start the llama.cpp router (no-op when it is already running)",
		Long: `Start the llama.cpp router, detached, and return once it answers /health.

The router is a small always-on process: it loads a model on the first
request and unloads it after 15 idle minutes. Use --foreground to run it
attached to this terminal (for debugging).`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := a.load(); err != nil {
				return err
			}
			ctx := cmd.Context()
			if foreground {
				return serveForeground(ctx, a)
			}
			ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
			defer cancel()
			wasUp := a.rt.Healthy(ctx)
			st, err := a.syncAndEnsureRouter(ctx)
			if err != nil {
				return err
			}
			status := "already running"
			switch {
			case st.Restarted:
				status = "restarted (models changed)"
			case st.Started:
				status = "started"
			}
			doc := output.Obj{}.Set("router", "up").Set("status", status).Set("port", a.settings.Port)
			if wasUp && st.Stale {
				doc = doc.Set("warning", "router predates your latest model changes; run `pill stop --all` then `pill serve`")
			}
			a.printer.Emit(doc.Set("log", a.paths.ServerLog()))
			return nil
		},
	}
	c.Flags().BoolVar(&foreground, "foreground", false, "run attached to this terminal instead of detached")
	return c
}

func serveForeground(ctx context.Context, a *App) error {
	if a.rt.Healthy(ctx) {
		return output.Fail([]string{"run `pill stop --all` first"}, "a router is already running on port %d", a.settings.Port)
	}
	snap, err := a.reg.Load()
	if err != nil {
		return output.Fail(nil, "%v", err)
	}
	if _, err := a.reg.Apply(snap); err != nil {
		return output.Fail(nil, "%v", err)
	}
	return output.Fail(nil, "cannot start llama-server: %v", a.rt.ExecForeground())
}
