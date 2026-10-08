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
	// Ownership first: unloading a foreign server's models would disrupt it
	// even though pill then refuses to stop it.
	if err := a.requireOwnRouter(ctx); err != nil {
		return err
	}
	doc := output.Obj{}
	unloaded := []string{}
	healthy := a.rt.Healthy(ctx)
	if healthy {
		got, err := a.rt.UnloadAll(ctx)
		switch {
		case err == nil:
			unloaded = append(unloaded, got...)
		case !all:
			return output.Fail([]string{"try `pill stop --all`"}, "%v", err)
		default:
			// --all is the escape hatch for a router that cannot unload cleanly:
			// carry on and stop the process itself.
			a.printer.Progress("could not unload cleanly (%v); stopping the router process", err)
		}
	}
	if !all {
		if !healthy {
			// Nothing answers: there is nothing to unload. Idempotent, exit 0.
			a.printer.Emit(doc.Set("router", "down").Set("unloaded", unloaded).
				Set("help", []string{"start it with `pill serve` or just run `pill`"}))
			return nil
		}
		a.printer.Emit(doc.Set("router", "up").Set("unloaded", unloaded))
		return nil
	}
	// --all stops the process (or boots the launchd job out) whether or not it
	// still answers /health: a starting, hung or crash-looping router is
	// exactly when someone runs this.
	stopped, err := a.stopRouter(ctx)
	if err != nil {
		return output.Fail(nil, "%v", err)
	}
	state := "stopped"
	if !stopped && !healthy {
		state = "down"
	}
	doc = doc.Set("router", state).Set("unloaded", unloaded)
	var help []string
	if a.svc.Installed() {
		help = append(help, "the LaunchAgent is unloaded until next login; bring it back with `pill serve`")
	} else if state == "down" {
		help = append(help, "start it with `pill serve` or just run `pill`")
	}
	a.printer.Emit(doc.Set("help", help))
	return nil
}

// stopRouter stops the router process however it is supervised.
//
// A router owned by the launchd service must be booted out: sending it
// SIGTERM would only make KeepAlive start it again.
func (a *App) stopRouter(ctx context.Context) (bool, error) {
	if a.serviceServesPort(ctx) { // a service on another port is not this router
		if err := a.svc.Stop(ctx); err != nil {
			return false, err
		}
		a.rt.WaitDown(ctx)
		return true, nil
	}
	return a.rt.Stop(ctx)
}
