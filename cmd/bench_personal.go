package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/marcellovictorino/pill/internal/bench"
	"github.com/marcellovictorino/pill/internal/config"
	"github.com/marcellovictorino/pill/internal/output"
	"github.com/marcellovictorino/pill/internal/pi"
)

// runPersonalBench is `pill bench run <name> --personal`: the same Tier 1 and
// Tier 2 tasks with a copy of the user's own Pi setup. It is informational and
// deliberately cannot reach the gate: it never calls the code that records a
// verdict, never saves results.json and never registers or removes a model, so
// it cannot change whether a model passes or whether Pi lists it.
func runPersonalBench(ctx context.Context, a *App, arg string, runs int, th config.Thresholds) error {
	lock, err := bench.Acquire(a.paths.BenchLock())
	if errors.Is(err, bench.ErrBusy) {
		return output.Fail([]string{"wait for it to finish, then run this again"}, "%v", err)
	} else if err != nil {
		return output.Fail(nil, "%v", err)
	}
	defer lock.Release()

	if inGitWorkTree(a.paths.PersonalDir()) {
		return output.Fail([]string{"unset PILL_HOME or point it outside every repository"},
			"refusing to write personal results inside a git work tree: %s", a.paths.PersonalDir())
	}
	snap, err := a.reg.Load()
	if err != nil {
		return output.Fail(nil, "%v", err)
	}
	m, ok := snap.Models.Find(arg)
	if !ok {
		return output.Fail([]string{"run it bare first: pill bench run " + arg}, "%s is not registered; a personal run compares against a bare one", arg)
	}
	if a.reg.FileSize(*m) == 0 {
		return output.Fail([]string{"download it first: pill bench run " + m.Name}, "%s is not on disk", m.Name)
	}
	piBin, err := pi.Binary()
	if err != nil {
		return output.Fail([]string{"install Pi: brew install pi-coding-agent"}, "%v", err)
	}
	if _, err := lookNode(); err != nil {
		return output.Fail([]string{"Node ships with Pi: brew install pi-coding-agent"}, "%v", err)
	}
	piDir, err := pi.AgentDir()
	if err != nil {
		return output.Fail(nil, "%v", err)
	}
	if _, err := os.Stat(piDir); err != nil {
		return output.Fail([]string{"a personal run copies your Pi folder; run Pi once to create it"}, "there is no Pi folder at %s", piDir)
	}

	// The router has to know the model, so models.ini is brought up to date; Pi's
	// provider and results.json are left alone.
	applied, err := a.reg.ApplyRouter(snap)
	if err != nil {
		return output.Fail(nil, "%v", err)
	}
	if err := a.startBenchRouter(ctx, applied); err != nil {
		return err
	}
	runner := a.newBenchRunner(piBin, lock)
	runner.PersonalSource = piDir

	stamp := time.Now().UTC().Format("20060102T150405Z")
	plan := bench.Plan{Model: *m, Runs: runs, Thresholds: th, Dir: filepath.Join(a.paths.PersonalBenchmarkDir(), stamp+"-"+m.Name)}
	a.printer.Progress("personal benchmark of %s (%d run(s)) with a copy of %s; results in %s", m.Name, runs, piDir, plan.Dir)
	res, err := runner.Run(ctx, plan)
	if err != nil {
		if ctx.Err() != nil {
			return output.Fail([]string{"nothing was recorded; run the benchmark again"}, "interrupted")
		}
		return output.Fail(nil, "%v", err)
	}
	rec := config.PersonalResult{Result: res, Personal: true, SetupHash: runner.SetupHash}
	if err := config.AppendPersonal(a.paths, rec); err != nil {
		return output.Fail([]string{"the full result is in " + plan.Dir}, "%v", err)
	}
	bare, _ := bench.LoadResults(a.paths.BenchmarkDir())
	rows := bench.SummarizePersonal([]config.PersonalResult{rec}, bare)
	a.printer.Emit(personalRowDoc(rows[0]).Set("dir", plan.Dir).
		Set("help", []string{"compare with the bare run: pill bench summary --personal", "personal results stay on this computer; do not commit or quote them"}))
	return nil
}

// personalRowDoc renders one personal row. There is no pass or fail on it.
func personalRowDoc(r bench.PersonalRow) output.Obj {
	return output.Obj{}.Set("model", r.Model).Set("result", "personal (informational)").
		Set("setup", r.SetupHash).Set("runs", r.Runs).
		Set("tool_check", r.ToolCheck).Set("tool_s", fmt.Sprintf("%.1f", r.ToolSecs)).Set("extra_s", r.ExtraSecs).
		Set("t2", r.Tier2).Set("t2_min", fmt.Sprintf("%.1f", r.Tier2Secs/60)).
		Set("bare_t2", r.BareTier2).Set("bare_t2_min", fmt.Sprintf("%.1f", r.BareSecs/60))
}

// personalSummary is `pill bench summary --personal`.
func personalSummary(a *App) error {
	personal, err := config.LoadPersonal(a.paths)
	if err != nil {
		return output.Fail(nil, "%v", err)
	}
	bare, err := bench.LoadResults(a.paths.BenchmarkDir())
	if err != nil {
		return output.Fail(nil, "%v", err)
	}
	rows := bench.SummarizePersonal(personal, bare)
	table := make([]output.Obj, 0, len(rows))
	for _, r := range rows {
		table = append(table, output.Obj{}.Set("model", r.Model).Set("setup", r.SetupHash).Set("runs", r.Runs).
			Set("tool_check", r.ToolCheck).Set("tool_s", fmt.Sprintf("%.1f", r.ToolSecs)).Set("extra_s", r.ExtraSecs).
			Set("t2", r.Tier2).Set("t2_min", fmt.Sprintf("%.1f", r.Tier2Secs/60)).
			Set("bare_t2", r.BareTier2).Set("bare_t2_min", fmt.Sprintf("%.1f", r.BareSecs/60)).
			Set("date", r.Date))
	}
	var help []string
	if len(rows) == 0 {
		help = append(help, "run your own Pi setup against a model: pill bench run <name> --personal")
	} else {
		help = append(help, "informational: a personal result has no pass or fail; extra_s is the Pi tool turn's time minus the bare run's, bare_t2 is the baseline")
	}
	a.printer.Emit(output.Obj{}.Set("personal", table).Set("help", help))
	return nil
}

// inGitWorkTree reports whether path (which may not exist yet) lies inside a
// git work tree, where a commit could pick personal results up.
func inGitWorkTree(path string) bool {
	p := path
	for {
		if _, err := os.Stat(p); err == nil {
			break
		}
		parent := filepath.Dir(p)
		if parent == p {
			return false
		}
		p = parent
	}
	out, err := exec.Command("git", "-C", p, "rev-parse", "--is-inside-work-tree").Output()
	return err == nil && strings.TrimSpace(string(out)) == "true"
}
