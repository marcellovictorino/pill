package cmd

import (
	"context"
	"time"

	"github.com/spf13/cobra"

	"github.com/marcellovictorino/pill/internal/output"
)

func newPsCmd(a *App) *cobra.Command {
	return &cobra.Command{
		Use:   "ps",
		Short: "Show the router, its port, and the loaded model with its memory use",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := a.load(); err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
			defer cancel()
			return runPs(ctx, a)
		},
	}
}

func runPs(ctx context.Context, a *App) error {
	doc := output.Obj{}
	if !a.rt.Healthy(ctx) {
		a.printer.Emit(doc.Set("router", "down").Set("port", a.settings.Port).
			Set("help", []string{"start it with `pill serve` or just run `pill`"}))
		return nil
	}
	mode := a.routerMode(ctx)
	proc, found := a.rt.Find()
	if found && !proc.Owned {
		mode = "external" // answering, but not pill's models.ini
	}
	doc = doc.Set("router", "up").Set("port", a.settings.Port).Set("mode", mode)
	if found {
		doc = doc.Set("pid", proc.PID)
	}
	loaded, err := a.rt.Loaded(ctx)
	if err != nil {
		return output.Fail(nil, "%v", err)
	}
	doc = doc.Set("default", orDash(a.settings.Default))
	if len(loaded) == 0 {
		doc = doc.Set("loaded", "none")
	} else {
		doc = doc.Set("loaded", joinIDs(loaded))
		if found {
			if rss, err := a.deps.Sys.ProcessTreeRSSBytes(proc.PID); err == nil && rss > 0 {
				doc = doc.Set("rss", humanBytes(rss))
			}
		}
		// Weights sit in Metal buffers that the resident size does not show.
		if s, err := a.deps.Sys.Sample(); err == nil && s.GPUMemBytes > 0 {
			doc = doc.Set("gpu_memory", humanBytes(s.GPUMemBytes))
		}
	}
	a.printer.Emit(doc)
	return nil
}

func orDash(s string) string {
	if s == "" {
		return "none"
	}
	return s
}
