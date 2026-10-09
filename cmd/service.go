package cmd

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/marcellovictorino/pill/internal/output"
	"github.com/marcellovictorino/pill/internal/router"
	"github.com/marcellovictorino/pill/internal/service"
)

func newServiceCmd(a *App) *cobra.Command {
	c := &cobra.Command{
		Use:   "service",
		Short: "Run the router as a launchd LaunchAgent (starts at login, restarts if it dies)",
		Args:  cobra.NoArgs,
		RunE:  func(cmd *cobra.Command, args []string) error { return cmd.Help() },
	}
	c.AddCommand(
		&cobra.Command{
			Use:   "install",
			Short: "Install and start the LaunchAgent",
			Long: `Write ~/Library/LaunchAgents/` + service.Label + `.plist and load it. The router then starts at
every login and is restarted if it dies. The agent runs llama-server directly, so
every other pill command keeps working unchanged. A detached router started by
'pill serve' is replaced (install refuses while it has a model loaded).`,
			Args: cobra.NoArgs,
			RunE: func(cmd *cobra.Command, args []string) error {
				if err := a.load(); err != nil {
					return err
				}
				ctx, cancel := context.WithTimeout(cmd.Context(), 3*time.Minute)
				defer cancel()
				return runServiceInstall(ctx, a)
			},
		},
		&cobra.Command{
			Use:   "uninstall",
			Short: "Stop and remove the LaunchAgent",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, args []string) error {
				if err := a.load(); err != nil {
					return err
				}
				ctx, cancel := context.WithTimeout(cmd.Context(), time.Minute)
				defer cancel()
				return runServiceUninstall(ctx, a)
			},
		},
	)
	return c
}

func runServiceInstall(ctx context.Context, a *App) error {
	bin, err := router.LlamaServer()
	if err != nil {
		return output.Fail([]string{"install llama.cpp: brew install llama.cpp"}, "%v", err)
	}
	_, applied, err := a.reg.Refresh(ctx) // the agent reads models.ini on every start
	if err != nil {
		return wrapState(err)
	}

	owned := a.serviceServesPort(ctx)
	// Whatever is replaced, check first that nothing is in use and that the
	// destination port is free, so a refusal leaves the old service and its
	// plist exactly as they were.
	if err := a.requireIdleServiceRouter(ctx); err != nil {
		return err
	}
	if !owned && a.rt.PortBusy() && !a.rt.Healthy(ctx) {
		return output.Fail([]string{"free the port or give pill another one with PILL_PORT"},
			"port %d is in use by another process that does not answer /health", a.settings.Port)
	}
	if a.rt.Healthy(ctx) && !owned {
		loaded, _ := a.rt.Loaded(ctx)
		if len(loaded) > 0 {
			return output.Fail([]string{"free it first: pill stop"}, "the router has %s loaded; not replacing it while it may be in use", joinIDs(loaded))
		}
		if _, err := a.rt.Stop(ctx); err != nil { // hand the port over to launchd
			return output.Fail([]string{"stop it yourself or run `pill stop --all`"}, "%v", err)
		}
		a.rt.WaitDown(ctx)
	}
	plist, err := a.svc.Install(ctx, bin, a.rt.Args(), launchPath(bin))
	if err != nil {
		return output.Fail([]string{"check the log: " + a.paths.ServerLog(), "undo with `pill service uninstall`"}, "%v", err)
	}
	if err := a.rt.WaitHealthy(ctx); err != nil {
		return output.Fail([]string{"undo with `pill service uninstall`"}, "%v", err)
	}
	a.rt.RecordStart(applied.IniHash) // so a later change to models.ini is noticed
	a.printer.Emit(output.Obj{}.
		Set("service", "installed").
		Set("label", service.Label).
		Set("plist", plist).
		Set("router", "up").
		Set("port", a.settings.Port).
		Set("log", a.paths.ServerLog()).
		Set("help", []string{"remove it again: pill service uninstall", "after `pill stop --all` the router stays down until next login or `pill serve`"}))
	return nil
}

func runServiceUninstall(ctx context.Context, a *App) error {
	wasLoaded := a.svc.Loaded(ctx)
	was, err := a.svc.Uninstall(ctx)
	if err != nil {
		return output.Fail(nil, "%v", err)
	}
	if wasLoaded { // booting the job out stops its router; an unrelated detached router stays up
		a.rt.WaitDown(ctx)
	}
	status := "removed"
	if !was {
		status = "not installed (nothing to remove)"
	}
	a.printer.Emit(output.Obj{}.Set("service", status).
		Set("help", []string{"start a detached router: pill serve"}))
	return nil
}

// launchPath is the PATH written into the LaunchAgent. It is deliberately not
// a copy of the current shell's PATH (which may hold temporary or per-project
// directories): launchd's default is minimal, so the directory of llama-server
// plus the usual Homebrew and system directories are enough.
func launchPath(llamaServer string) string {
	return strings.Join([]string{
		filepath.Dir(llamaServer), "/opt/homebrew/bin", "/usr/local/bin", "/usr/bin", "/bin", "/usr/sbin", "/sbin",
	}, ":")
}
