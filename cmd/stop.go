package cmd

import (
	"context"
	"time"

	"github.com/spf13/cobra"

	"github.com/marcellovictorino/pill/internal/output"
)

func newStopCmd(a *App) *cobra.Command {
	var all bool
	c := &cobra.Command{
		Use:   "stop",
		Short: "Unload the loaded model (--all also stops the router)",
		Long: `Unload the loaded model to free its memory. The router stays up and
reloads the model on the next request.

With --all the router itself is stopped as well.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := a.load(); err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 90*time.Second)
			defer cancel()
			return runStop(ctx, a, all)
		},
	}
	c.Flags().BoolVar(&all, "all", false, "also stop the router process")
	return c
}

func runStop(ctx context.Context, a *App, all bool) error {
	doc := output.Obj{}
	if !a.rt.Healthy(ctx) {
		// Nothing answers: stopping is already done. Idempotent, exit 0.
		a.printer.Emit(doc.Set("router", "down").Set("unloaded", []string{}).
			Set("help", []string{"start it with `pill serve` or just run `pill`"}))
		return nil
	}
	unloaded, err := a.rt.UnloadAll(ctx)
	if err != nil {
		return output.Fail([]string{"try `pill stop --all`"}, "%v", err)
	}
	if unloaded == nil {
		unloaded = []string{}
	}
	if !all {
		a.printer.Emit(doc.Set("router", "up").Set("unloaded", unloaded))
		return nil
	}
	if _, err := a.stopRouter(ctx); err != nil {
		return output.Fail(nil, "%v", err)
	}
	doc = doc.Set("router", "stopped").Set("unloaded", unloaded)
	if a.svc.Installed() {
		doc = doc.Set("help", []string{"the LaunchAgent is unloaded until next login; bring it back with `pill serve`"})
	}
	a.printer.Emit(doc)
	return nil
}

// stopRouter stops the router process however it is supervised.
//
// A router owned by the launchd service must be booted out: sending it
// SIGTERM would only make KeepAlive start it again.
func (a *App) stopRouter(ctx context.Context) (bool, error) {
	if a.svc.Installed() && a.svc.Loaded(ctx) {
		if err := a.svc.Stop(ctx); err != nil {
			return false, err
		}
		a.rt.WaitDown(ctx)
		return true, nil
	}
	return a.rt.Stop(ctx)
}
