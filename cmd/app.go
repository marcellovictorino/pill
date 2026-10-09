// Package cmd holds pill's cobra commands, one file per command.
//
// cobra (github.com/spf13/cobra) is the command-line framework behind
// kubectl, gh and Hugo: each command is a *cobra.Command value with a Use
// line, flags, and a RunE function. Commands are built by constructor
// functions (newServeCmd, ...) that receive the shared *App, which keeps
// state out of package globals and makes the commands easy to test.
package cmd

import (
	"context"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/marcellovictorino/pill/internal/config"
	"github.com/marcellovictorino/pill/internal/output"
	"github.com/marcellovictorino/pill/internal/registry"
	"github.com/marcellovictorino/pill/internal/router"
	"github.com/marcellovictorino/pill/internal/service"
	"github.com/marcellovictorino/pill/internal/sysinfo"
)

// Deps are the things tests replace: output streams, the terminal check and
// the platform layer.
type Deps struct {
	Out, Err  io.Writer
	In        io.Reader // stdin, used only for the one interactive question (demoting a model)
	StdoutTTY bool
	StdinTTY  bool
	Sys       sysinfo.Info
}

// App is the state shared by all commands. The printer is set once flags are
// parsed; paths, settings and the registry load lazily so that `pill --help`
// and `pill version` work even when the configuration is broken.
type App struct {
	deps    Deps
	format  string
	printer *output.Printer

	paths    config.Paths
	settings config.Settings
	reg      *registry.Registry
	rt       *router.Router
	svc      *service.Service
	loaded   bool
}

// snapshot is the registry's models.toml + results.json view.
type snapshot = registry.Snapshot

// P is the printer.
func (a *App) P() *output.Printer { return a.printer }

// load resolves paths and settings on first use.
func (a *App) load() error {
	if a.loaded {
		return nil
	}
	p, err := config.Resolve()
	if err != nil {
		return output.Fail(nil, "%v", err)
	}
	s, err := config.Load(p)
	if err != nil {
		return output.Fail([]string{"fix or remove the file, or unset PILL_PORT"}, "%v", err)
	}
	reg, err := registry.Open(p, s)
	if err != nil {
		return output.Fail(nil, "%v", err)
	}
	a.paths, a.settings, a.reg = p, s, reg
	a.rt = router.New(p, s)
	a.svc = service.New(p)
	a.loaded = true
	return nil
}

// Execute runs pill and returns the process exit code.
func Execute() int {
	deps := Deps{
		Out: os.Stdout, Err: os.Stderr, In: os.Stdin,
		StdoutTTY: output.IsTerminal(os.Stdout),
		StdinTTY:  output.IsTerminal(os.Stdin),
		Sys:       sysinfo.New(),
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return Run(ctx, deps, os.Args[1:])
}

// Run executes one pill invocation with explicit dependencies; tests call it
// directly.
func Run(ctx context.Context, deps Deps, args []string) int {
	root, app := newRoot(deps)
	if args == nil {
		args = []string{} // SetArgs(nil) would make cobra read os.Args instead
	}
	root.SetArgs(args)
	err := root.ExecuteContext(ctx)
	if err == nil {
		return output.ExitOK
	}
	if app.printer == nil {
		// Failure before flags were parsed (bad --format, unknown flag on root).
		p, _ := output.NewPrinter(deps.Out, deps.Err, "", deps.StdoutTTY)
		app.printer = p
	}
	e, ok := err.(*output.Error)
	if !ok {
		e = classify(root, err)
	}
	app.printer.Fail(e)
	return e.Code
}

// classify turns cobra's plain errors into pill errors: argument and command
// mistakes are usage errors (exit 2) with the valid choices listed.
func classify(root *cobra.Command, err error) *output.Error {
	msg := err.Error()
	for _, prefix := range []string{"unknown command", "accepts ", "requires ", "invalid argument", "required flag"} {
		if strings.HasPrefix(msg, prefix) {
			help := []string{"run `pill --help` for usage"}
			if strings.HasPrefix(msg, "unknown command") {
				help = validFlagsHelp(root)[1:]
			}
			return output.Usage(help, "%s", msg)
		}
	}
	return output.Fail(nil, "%s", msg)
}
