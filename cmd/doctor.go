package cmd

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/marcellovictorino/pill/internal/output"
	"github.com/marcellovictorino/pill/internal/pi"
	"github.com/marcellovictorino/pill/internal/router"
	"github.com/marcellovictorino/pill/internal/service"
)

// check is one doctor line.
type check struct {
	name   string
	status string // ok, warn, fail
	detail string
}

func newDoctorCmd(a *App) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check prerequisites, the router, and Pi's configuration",
		Long: `Check that llama-server and Pi are installed, the port is free or owned by
pill's router, the router is healthy, Pi's "pill" provider matches models.toml,
and no GGUF files are hiding in the config directory.

Exits 1 only when something is broken (a failing check); warnings exit 0.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := a.load(); err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			return runDoctor(ctx, a)
		},
	}
}

func runDoctor(ctx context.Context, a *App) error {
	var checks []check
	add := func(name, status, detail string) { checks = append(checks, check{name, status, detail}) }
	var help []string

	// Prerequisites: pill never installs them.
	if bin, err := router.LlamaServer(); err != nil {
		add("llama-server", "fail", "not found on PATH")
		help = append(help, "install llama.cpp: brew install llama.cpp")
	} else {
		add("llama-server", "ok", fmt.Sprintf("%s %s", bin, llamaVersion(bin)))
	}
	if bin, err := pi.Binary(); err != nil {
		add("pi", "fail", "not found on PATH")
		help = append(help, "install Pi: brew install pi-coding-agent")
	} else if v, err := pi.Version(bin); err != nil {
		add("pi", "warn", bin+" (version unknown)")
	} else {
		add("pi", "ok", bin+" "+v)
	}

	// Port and router.
	healthy := a.rt.Healthy(ctx)
	proc, found := a.rt.Find()
	switch {
	case !a.rt.PortBusy():
		add("port", "ok", fmt.Sprintf("%d is free", a.settings.Port))
	case found && proc.Owned:
		add("port", "ok", fmt.Sprintf("%d is held by pill's router (pid %d)", a.settings.Port, proc.PID))
	default:
		add("port", "fail", fmt.Sprintf("%d is in use by another process (pid %d)", a.settings.Port, proc.PID))
		help = append(help, "free the port or set PILL_PORT / port in config.toml")
	}
	if healthy {
		add("router", "ok", "answering /health")
	} else {
		add("router", "ok", "not running (it starts on demand)")
	}

	switch {
	case !a.svc.Installed():
		add("service", "ok", "not installed (optional: pill service install)")
	case a.serviceCommandStale():
		add("service", "warn", "LaunchAgent was installed with different settings (port, paths) than the current ones")
		help = append(help, "refresh it: pill service install")
	case a.svc.Loaded(ctx):
		add("service", "ok", "LaunchAgent "+service.Label+" is loaded")
	default:
		add("service", "warn", "LaunchAgent is installed but not loaded (stopped with `pill stop --all`)")
		help = append(help, "load it again: pill serve")
	}

	snap, err := a.reg.Load()
	if err != nil {
		return output.Fail(nil, "%v", err)
	}

	// Models.
	if len(snap.Models.Models) == 0 {
		add("models", "warn", "none registered")
		help = append(help, "register one: pill add gemma4-26b-iq4xs-64k --unverified")
	} else {
		var missing []string
		for _, m := range snap.Models.Models {
			if a.reg.FileSize(m) == 0 {
				missing = append(missing, m.Name)
			}
		}
		if len(missing) > 0 {
			add("models", "warn", "declared but missing locally: "+strings.Join(missing, " "))
			help = append(help, "fetch them: pill sync (or copy the GGUFs into "+a.settings.ModelsDir+")")
		} else {
			add("models", "ok", fmt.Sprintf("%d registered, all files present", len(snap.Models.Models)))
		}
	}
	if d := a.settings.Default; d == "" {
		add("default", "warn", "no default model set")
		help = append(help, "pill default <name>")
	} else if m, ok := snap.Models.Find(d); !ok || a.reg.FileSize(*m) == 0 {
		add("default", "fail", d+" is not registered or its file is missing")
	} else {
		add("default", "ok", d)
	}

	// Pi's provider in sync with models.toml.
	modelsJSON := pi.ModelsPath(a.reg.PiDir)
	provider, _, perr := pi.ReadProvider(modelsJSON)
	want := ids(a.reg.PiModels(snap))
	switch {
	case perr != nil:
		add("pi-provider", "fail", perr.Error())
	case provider == nil && len(want) == 0:
		add("pi-provider", "ok", "no pill provider (nothing registered)")
	case provider == nil:
		add("pi-provider", "warn", "pill provider missing from "+modelsJSON)
		help = append(help, "write it: pill serve (or any pill add)")
	default:
		have := pi.ProviderModelIDs(provider)
		sort.Strings(have)
		wantBase := fmt.Sprintf("http://127.0.0.1:%d/v1", a.settings.Port)
		if strings.Join(have, ",") != strings.Join(want, ",") || pi.ProviderBaseURL(provider) != wantBase {
			add("pi-provider", "warn", "out of sync with models.toml")
			help = append(help, "re-sync: pill serve")
		} else {
			add("pi-provider", "ok", "in sync: "+strings.Join(want, " "))
		}
	}

	if notes := a.olderBuildNotes(snap); len(notes) > 0 {
		var names []string
		for name := range notes {
			names = append(names, name)
		}
		sort.Strings(names)
		add("bench-freshness", "warn", "benched on a different llama.cpp build: "+strings.Join(names, " "))
		help = append(help, "re-run: pill bench run <name> (a pass is not invalidated by an upgrade, but may have changed)")
	}

	// Hint about a leftover local-router provider; never remove it.
	if others, err := pi.OtherProviders(modelsJSON); err == nil {
		for _, key := range sortedKeys(others) {
			if base := others[key]; key == "llama-cpp" || strings.Contains(base, "127.0.0.1:8080") || strings.Contains(base, "localhost:8080") {
				add("legacy-provider", "warn", fmt.Sprintf("Pi provider %q points at an old local server (%s)", key, base))
				help = append(help, "remove it from "+modelsJSON+" if you no longer use it (pill never edits other providers)")
			}
		}
	}

	// GGUFs in the portable config directory would end up in dotfiles.
	if ggufs := ggufsIn(a.paths.ConfigDir); len(ggufs) > 0 {
		add("config-dir", "warn", fmt.Sprintf("%s contains %d .gguf file(s); keep GGUFs in %s", a.paths.ConfigDir, len(ggufs), a.settings.ModelsDir))
	} else {
		add("config-dir", "ok", "no GGUF files in "+a.paths.ConfigDir)
	}

	status := "ok"
	for _, c := range checks {
		if c.status == "fail" {
			status = "fail"
			break
		}
		if c.status == "warn" {
			status = "warn"
		}
	}
	rows := make([]output.Obj, len(checks))
	for i, c := range checks {
		rows[i] = output.Obj{}.Set("check", c.name).Set("status", c.status).Set("detail", c.detail)
	}
	a.printer.Emit(output.Obj{}.Set("status", status).Set("checks", rows).Set("help", help))
	if status == "fail" {
		return &output.Error{Msg: "one or more checks failed", Code: output.ExitFailure, Help: nil}
	}
	return nil
}

func ids(models []pi.Model) []string {
	out := make([]string, len(models))
	for i, m := range models {
		out[i] = m.ID
	}
	sort.Strings(out)
	return out
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// llamaVersion runs `llama-server --version`; the first "version:" line looks
// like "version: 0.6.0 (build 11429, commit d81235049)".
func llamaVersion(bin string) string {
	out, _ := exec.Command(bin, "--version").CombinedOutput()
	for _, line := range strings.Split(string(out), "\n") {
		if v, ok := strings.CutPrefix(strings.TrimSpace(line), "version:"); ok {
			return strings.TrimSpace(v)
		}
	}
	return "(version unknown)"
}

func ggufsIn(dir string) []string {
	var found []string
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(strings.ToLower(d.Name()), ".gguf") {
			found = append(found, path)
		}
		return nil
	})
	return found
}

// lookNode finds node, which Pi depends on and the Tier 2 verifier needs.
func lookNode() (string, error) {
	if n := os.Getenv("PILL_NODE"); n != "" {
		return n, nil
	}
	return exec.LookPath("node")
}
