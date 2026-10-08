package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/marcellovictorino/pill/internal/output"
	"github.com/marcellovictorino/pill/internal/registry"
)

// routerStatus is what ensureRouter did.
type routerStatus struct {
	Started   bool // a new process was launched
	Restarted bool // an older router was replaced because models.ini changed
	Stale     bool // router runs with an older models.ini but could not be restarted
}

// syncAndEnsureRouter regenerates models.ini and Pi's provider, then makes
// sure a router answering the current models.ini is running.
func (a *App) syncAndEnsureRouter(ctx context.Context) (routerStatus, error) {
	var st routerStatus
	snap, err := a.reg.Load()
	if err != nil {
		return st, output.Fail(nil, "%v", err)
	}
	applied, err := a.reg.Apply(snap)
	if err != nil {
		return st, output.Fail(nil, "%v", err)
	}
	return a.ensureRouter(ctx, applied)
}

func (a *App) ensureRouter(ctx context.Context, applied registry.Applied) (routerStatus, error) {
	var st routerStatus
	rt := a.rt
	if rt.Healthy(ctx) {
		if !a.routerStale(ctx, applied) {
			return st, nil
		}
		loaded, _ := rt.Loaded(ctx)
		if len(loaded) > 0 {
			st.Stale = true // never kill a model somebody may be using
			return st, nil
		}
		if err := a.stopRouterProcess(ctx); err != nil {
			st.Stale = true
			return st, nil
		}
		st.Restarted = true
	}
	started, err := a.startRouterProcess(ctx, applied.IniHash)
	if err != nil {
		return st, output.Fail([]string{"check the log: " + a.paths.ServerLog(), "run `pill doctor`"}, "%v", err)
	}
	st.Started = started
	return st, nil
}

// routerStale reports whether the running router was started before the
// current models.ini. pill-started routers record the file's hash; for any
// other router (launchd, hand-started) the set of model ids is compared.
func (a *App) routerStale(ctx context.Context, applied registry.Applied) bool {
	if h := a.rt.ReadIniHash(); h != "" {
		return h != applied.IniHash
	}
	models, err := a.rt.Models(ctx)
	if err != nil {
		return false
	}
	have := map[string]bool{}
	for _, m := range models {
		have[m.ID] = true
	}
	for _, id := range applied.Served {
		if !have[id] {
			return true
		}
	}
	return false
}

func (a *App) startRouterProcess(ctx context.Context, iniHash string) (bool, error) {
	return a.rt.Start(ctx, iniHash)
}

func (a *App) stopRouterProcess(ctx context.Context) error {
	_, err := a.rt.Stop(ctx)
	return err
}

// routerMode says how the router is supervised: "service" (launchd) or
// "detached" (started by `pill serve`).
func (a *App) routerMode() string { return "detached" }

func joinIDs(ids []string) string {
	if len(ids) == 0 {
		return "none"
	}
	return strings.Join(ids, ", ")
}

// humanBytes formats a size in decimal units (1 GB = 10^9 bytes), matching
// Hugging Face and the catalog so the same file never shows two sizes.
func humanBytes(n int64) string {
	const kb, mb, gb = 1e3, 1e6, 1e9
	switch f := float64(n); {
	case f >= gb:
		return fmt.Sprintf("%.1f GB", f/gb)
	case f >= mb:
		return fmt.Sprintf("%.0f MB", f/mb)
	case f >= kb:
		return fmt.Sprintf("%.0f KB", f/kb)
	}
	return fmt.Sprintf("%d B", n)
}
