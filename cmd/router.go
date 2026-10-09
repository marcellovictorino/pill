package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/marcellovictorino/pill/internal/config"
	"github.com/marcellovictorino/pill/internal/output"
	"github.com/marcellovictorino/pill/internal/registry"
	"github.com/marcellovictorino/pill/internal/router"
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
	_, applied, err := a.reg.Refresh(ctx)
	if err != nil {
		return st, wrapState(err)
	}
	return a.ensureRouter(ctx, applied)
}

func (a *App) ensureRouter(ctx context.Context, applied registry.Applied) (routerStatus, error) {
	var st routerStatus
	rt := a.rt
	if rt.Healthy(ctx) {
		if !a.routerStale(ctx, applied) && !a.serviceOutdated(ctx) {
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
// current models.ini. A router pill started (detached, or through launchd and
// recorded afterwards) knows the fingerprint of the file it read. For any
// other router the model list is compared in both directions: a model the
// router lacks, or one it still offers that is no longer defined.
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
	want := map[string]bool{}
	for _, id := range applied.Served {
		want[id] = true
		if !have[id] {
			return true
		}
	}
	// The router also lists every GGUF in the models directory under its file
	// stem; those are not pill entries, so only other ids can be leftovers.
	raw := map[string]bool{}
	if files, err := os.ReadDir(a.settings.ModelsDir); err == nil {
		for _, f := range files {
			raw[strings.TrimSuffix(f.Name(), filepath.Ext(f.Name()))] = true
		}
	}
	for id := range have {
		if !want[id] && !raw[id] {
			return true
		}
	}
	return false
}

// wrapState turns a failure from registry.Update into a pill error: pill's own
// errors pass through, anything else (the lock timing out, a file error) gets
// a short explanation.
func wrapState(err error) error {
	var e *output.Error
	if errors.As(err, &e) {
		return err
	}
	return output.Fail(nil, "%v", err)
}

// requireOwnRouter refuses to go on when the configured port is answered by a
// llama-server that pill did not start: unloading its models or stopping it
// would disrupt somebody else's server.
func (a *App) requireOwnRouter(ctx context.Context) error {
	// Ask the operating system who holds the port, whatever /health says: a
	// server that is busy or recovering still belongs to somebody else.
	if p, ok := a.rt.Find(); ok && !p.Owned {
		return output.Fail([]string{"stop that server yourself, or give pill another port with PILL_PORT"},
			"port %d is served by a llama-server that pill did not start (pid %d); pill will not touch it", a.settings.Port, p.PID)
	}
	return nil
}

// serviceWant is the command the LaunchAgent should run with the current
// settings; it is what `pill service install` writes into the plist.
func (a *App) serviceWant() []string {
	bin, err := router.LlamaServer()
	if err != nil {
		return nil
	}
	return append([]string{bin}, a.rt.Args()...)
}

// installedPort is the port the installed LaunchAgent's router listens on
// (0 when none is installed or the plist has no --port).
func (a *App) installedPort() int {
	cmd := a.svc.InstalledCommand()
	for i := 0; i+1 < len(cmd); i++ {
		if cmd[i] == "--port" {
			n, _ := strconv.Atoi(cmd[i+1])
			return n
		}
	}
	return 0
}

// serviceServesPort reports whether the loaded LaunchAgent is the router for
// the configured port. With PILL_PORT pointing elsewhere the service is
// somebody else's business: stopping or replacing it would disrupt the router
// on the other port.
func (a *App) serviceServesPort(ctx context.Context) bool {
	return a.svc.Installed() && a.svc.Loaded(ctx) && a.installedPort() == a.settings.Port
}

// serviceOutdated reports whether the router on this port is the service and
// its saved command no longer matches the settings (idle time, models
// directory), so it needs the plist rewritten even if models.ini is current.
func (a *App) serviceOutdated(ctx context.Context) bool {
	return a.serviceServesPort(ctx) && a.serviceCommandStale()
}

// serviceCommandStale reports whether the installed plist's command differs
// from what the current settings would write.
func (a *App) serviceCommandStale() bool {
	return !slices.Equal(a.svc.InstalledCommand(), a.serviceWant())
}

// startRouterProcess brings the router up: through launchd when the service
// is installed, otherwise as a detached process.
func (a *App) startRouterProcess(ctx context.Context, iniHash string) (bool, error) {
	if !a.svc.Installed() {
		return a.rt.Start(ctx, iniHash)
	}
	bin, err := router.LlamaServer()
	if err != nil {
		return false, err
	}
	// Whatever is on the port and does not answer /health blocks both a start
	// and a replacement: launchd could not bind the port.
	if a.rt.PortBusy() && !a.rt.Healthy(ctx) {
		return false, fmt.Errorf("port %d is in use by another process that does not answer /health", a.settings.Port)
	}
	before, hadBefore := a.rt.Find()
	// The saved definition fixes the port and paths. If settings changed since
	// `pill service install` (config.toml, PILL_PORT), launchd would keep
	// starting a router on the old port while pill waits on the new one:
	// rewrite and reload the job first.
	if a.serviceCommandStale() {
		if err := a.requireIdleServiceRouter(ctx); err != nil {
			return false, err
		}
		if _, err := a.svc.Install(ctx, bin, a.rt.Args(), launchPath(bin)); err != nil {
			return false, err
		}
	} else if err := a.svc.Start(ctx, false); err != nil {
		return false, err
	}
	if err := a.rt.WaitHealthy(ctx); err != nil {
		return false, err
	}
	// Record the fingerprint only for a process that really started from this
	// models.ini: launchd may have kept an older one running.
	if after, ok := a.rt.Find(); ok && (!hadBefore || after.PID != before.PID) {
		a.rt.RecordStart(iniHash)
	}
	return true, nil
}

// requireIdleServiceRouter refuses to replace the installed LaunchAgent while
// its router (on the port saved in the plist, which may differ from the
// current one) may be in use: a model is loaded, or the router is there but
// does not answer, so that it cannot be shown to be idle.
func (a *App) requireIdleServiceRouter(ctx context.Context) error {
	port := a.installedPort()
	if port == 0 || !a.svc.Loaded(ctx) {
		return nil
	}
	old := router.New(a.paths, config.Settings{Port: port, IdleSeconds: a.settings.IdleSeconds, ModelsDir: a.settings.ModelsDir})
	if !old.Healthy(ctx) {
		if old.PortBusy() {
			return output.Fail([]string{fmt.Sprintf("stop it first: PILL_PORT=%d pill stop --all", port)},
				"the installed service's router on port %d does not answer /health, so pill cannot tell whether it is in use; not replacing it", port)
		}
		return nil // nothing is running there
	}
	loaded, err := old.Loaded(ctx)
	if err != nil {
		return output.Fail([]string{fmt.Sprintf("stop it first: PILL_PORT=%d pill stop --all", port)},
			"cannot tell whether the installed service's router on port %d is in use: %v", port, err)
	}
	if len(loaded) > 0 {
		return output.Fail([]string{fmt.Sprintf("free it first: PILL_PORT=%d pill stop", port)},
			"the installed service's router on port %d has %s loaded; not replacing it while it may be in use", port, joinIDs(loaded))
	}
	return nil
}

func (a *App) stopRouterProcess(ctx context.Context) error {
	_, err := a.stopRouter(ctx)
	return err
}

// routerMode says how the router is supervised: "service" (launchd) or
// "detached" (started by `pill serve`).
func (a *App) routerMode(ctx context.Context) string {
	if a.serviceServesPort(ctx) {
		return "service"
	}
	return "detached"
}

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
