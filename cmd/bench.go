package cmd

import (
	"bufio"
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/marcellovictorino/pill/internal/bench"
	"github.com/marcellovictorino/pill/internal/config"
	"github.com/marcellovictorino/pill/internal/output"
	"github.com/marcellovictorino/pill/internal/pi"
	"github.com/marcellovictorino/pill/internal/preset"
	"github.com/marcellovictorino/pill/internal/router"
	"github.com/marcellovictorino/pill/internal/sysinfo"
	"github.com/marcellovictorino/pill/internal/version"
)

func newBenchCmd(a *App) *cobra.Command {
	c := &cobra.Command{
		Use:   "bench",
		Short: "Benchmark models on this machine and rank the results",
		Long: `Benchmark models on this machine. A model that passes is marked "passed" and
registered with Pi; one that fails is marked "failed" and never reaches Pi.

Use 'pill bench run <name>' to measure and 'pill bench summary' to compare.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error { return cmd.Help() },
	}
	c.AddCommand(newBenchRunCmd(a), newBenchSummaryCmd(a))
	return c
}

func newBenchRunCmd(a *App) *cobra.Command {
	th := bench.DefaultThresholds()
	var runs, ctxSize int
	c := &cobra.Command{
		Use:   "run <name>",
		Short: "Run Tier 1 and Tier 2 on a model and apply the gate",
		Long: `Benchmark a model through the router, from a cold start.

Tier 1: an agent-sized request (~8K tokens) must finish within the time limit
without free memory dropping below the limit, and Pi must complete a
tool-calling turn correctly. Tier 2 (only if Tier 1 passed): Pi must build a
small spec-driven program, which a hidden verifier script then scores; a pass
needs the full score within the time cap. A pass needs every run to pass.

<name> is a catalog name, a registered model, or a Hugging Face reference; the
GGUF is downloaded if missing. Results are saved under ~/.pill/benchmark.
This takes a while (up to ~25 minutes per run).`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: a.completeModels(modelSources{catalog: true, registered: true}),
		RunE: func(cmd *cobra.Command, args []string) error {
			if runs < 1 {
				return output.Usage([]string{"--runs must be 1 or more"}, "--runs %d is not valid", runs)
			}
			if err := a.load(); err != nil {
				return err
			}
			return runBench(cmd.Context(), a, args[0], ctxSize, runs, th)
		},
	}
	c.Flags().IntVar(&runs, "runs", 1, "number of cold-start runs; a pass needs every run to pass")
	c.Flags().IntVar(&ctxSize, "ctx", 0, "context size when benchmarking a model that is not registered yet")
	c.Flags().Float64Var(&th.Tier1MaxSeconds, "tier1-max-seconds", th.Tier1MaxSeconds, "Tier 1: time limit for the agent-sized request")
	c.Flags().Float64Var(&th.MinFreePct, "min-free-pct", th.MinFreePct, "Tier 1: free memory must stay at or above this percentage")
	c.Flags().Float64Var(&th.Tier2MaxMinutes, "tier2-max-minutes", th.Tier2MaxMinutes, "Tier 2: time cap for building the task")
	return c
}

// benchOutcome is what one benchmark run decided.
type benchOutcome struct {
	Model config.Model
	Res   config.Result
	State string
	Dir   string
	Note  string
}

// benchModel pulls (when needed), registers and benchmarks one model and
// records the verdict. It prints nothing: runBench and `pill sync --bench`
// decide how to report.
func (a *App) benchModel(ctx context.Context, arg string, ctxSize, runs int, th config.Thresholds) (*benchOutcome, error) {
	snap, err := a.reg.Load()
	if err != nil {
		return nil, output.Fail(nil, "%v", err)
	}
	m, err := a.reg.Resolve(snap, arg, ctxSize)
	if err != nil {
		return nil, output.Fail([]string{"see the built-in models: pill catalog"}, "%v", err)
	}
	if a.reg.FileSize(m) == 0 { // pull if needed
		if m.Repo == "" {
			return nil, output.Fail([]string{"copy the GGUF into " + a.settings.ModelsDir}, "%s is not on disk and has no download source", m.Name)
		}
		file, _, _, err := a.pullModel(ctx, snap, m, remotePathOf(arg))
		if err != nil {
			return nil, err
		}
		m.File = file.Name()
	}
	piBin, err := pi.Binary()
	if err != nil {
		return nil, output.Fail([]string{"install Pi: brew install pi-coding-agent"}, "%v", err)
	}
	if _, err := lookNode(); err != nil {
		return nil, output.Fail([]string{"Node ships with Pi: brew install pi-coding-agent"}, "%v", err)
	}

	// Write the candidate into models.toml and models.ini as unverified. It
	// does not reach Pi unless it was already registered there.
	var prevState string
	loaded, hadEntry := snap.Models.Find(m.Name)
	var loadedCopy config.Model
	if hadEntry {
		loadedCopy = *loaded
	}
	_, applied, err := a.reg.UpdateApply(ctx, func(s *snapshot) error {
		// Resolving, and maybe downloading, took a while. Register the
		// candidate only if its entry is still what Resolve saw: otherwise a
		// concurrent rm would be undone, or an add overwritten.
		cur, has := s.Models.Find(m.Name)
		if has != hadEntry || (has && !sameModel(*cur, loadedCopy)) {
			return output.Fail([]string{"run the benchmark again: pill bench run " + arg}, "%s was changed while the benchmark was being prepared", m.Name)
		}
		s.Models.Upsert(m)
		s.Results.Ensure(m.Name)
		prevState = a.reg.State(s, m)
		return nil
	})
	if err != nil {
		return nil, wrapState(err)
	}
	// Benchmark through the main router: free whatever it has loaded, then make
	// sure it knows the candidate. Never unload a server pill did not start.
	if err := a.requireOwnRouter(ctx); err != nil {
		return nil, err
	}
	if a.rt.Healthy(ctx) {
		if _, err := a.rt.UnloadAll(ctx); err != nil {
			return nil, output.Fail([]string{"pill stop --all"}, "cannot unload the router's current model: %v", err)
		}
	}
	if _, err := a.ensureRouter(ctx, applied); err != nil {
		return nil, err
	}

	ramBytes, _ := a.deps.Sys.TotalRAMBytes()
	env := config.Environment{
		LlamaCpp: a.llamaVersion(), Pill: version.Version, MacOS: a.deps.Sys.OSVersion(), RAMGB: sysinfo.RAMGB(ramBytes),
	}
	env.Pi, _ = pi.Version(piBin)
	runner := bench.NewRunner(a.rt, a.deps.Sys, piBin, env)
	runner.Log = func(format string, args ...any) { a.printer.Progress(format, args...) }

	stamp := time.Now().UTC().Format("20060102T150405Z")
	plan := bench.Plan{Model: m, Runs: runs, Thresholds: th, Dir: filepath.Join(a.paths.BenchmarkDir(), stamp+"-"+m.Name)}
	a.printer.Progress("benchmarking %s (%d run(s)); results in %s", m.Name, runs, plan.Dir)
	res, err := runner.Run(ctx, plan)
	if err != nil {
		if ctx.Err() != nil {
			return nil, output.Fail([]string{"nothing was recorded; run the benchmark again"}, "interrupted")
		}
		return nil, output.Fail(nil, "%v", err)
	}

	// Record the verdict. The latest result decides the state. The prompt (if
	// any) comes first, outside the state lock, because it waits for a person.
	demote := true
	if !res.Passed && prevState == config.StatePassed {
		demote = a.confirmDemotion(m.Name)
	}
	var state, note string
	_, applied, err = a.reg.UpdateApply(ctx, func(s *snapshot) error {
		// The benchmark ran for minutes. Judge the entry as it is now: if it was
		// removed or redefined meanwhile (pill rm, pill add --ctx), the verdict
		// belongs to something that no longer exists, and writing it back would
		// resurrect or overwrite the other command's change.
		cur, ok := s.Models.Find(m.Name)
		switch {
		case !ok:
			return output.Fail([]string{"the full result is in " + plan.Dir}, "%s was removed while it was being benchmarked; its verdict was not recorded", m.Name)
		case !sameModel(*cur, m):
			return output.Fail([]string{"the full result is in " + plan.Dir, "benchmark it again: pill bench run " + m.Name}, "%s was changed while it was being benchmarked; its verdict was not recorded", m.Name)
		}
		st := s.Results.Ensure(m.Name)
		st.Latest = &res
		st.PresetHash = preset.SectionHash(m, a.settings.ModelsDir)
		st.GGUFSize = a.reg.FileSize(m)
		if res.Passed {
			st.State, st.InPi = config.StatePassed, true
		} else if !demote {
			note = "kept as passed: demotion declined"
		} else {
			st.State, st.InPi = config.StateFailed, false
		}
		state = st.State
		return nil
	})
	if err != nil {
		return nil, wrapState(err)
	}
	if a.rt.Healthy(ctx) {
		if _, err := a.ensureRouter(ctx, applied); err != nil {
			return nil, err
		}
	}

	return &benchOutcome{Model: m, Res: res, State: state, Dir: plan.Dir, Note: note}, nil
}

func runBench(ctx context.Context, a *App, arg string, ctxSize, runs int, th config.Thresholds) error {
	out, err := a.benchModel(ctx, arg, ctxSize, runs, th)
	if err != nil {
		return err
	}
	res, m := out.Res, out.Model
	doc := benchDoc(res, out.State, out.Dir, out.Note)
	var help []string
	switch {
	case res.Passed && a.settings.Default != m.Name:
		help = append(help, "make it the default: pill default "+m.Name)
	case !res.Passed && a.settings.Default == m.Name:
		help = append(help, "the default model failed: choose another with pill default <name>")
	}
	help = append(help, "compare models: pill bench summary")
	a.printer.Emit(doc.Set("help", help))
	if !res.Passed {
		return output.ExitWith(output.ExitFailure) // scripts can branch: bench run x && pill default x
	}
	return nil
}

// benchDoc renders a result for the terminal or an agent.
func benchDoc(res config.Result, state, dir, note string) output.Obj {
	verdict := "failed"
	if res.Passed {
		verdict = "passed"
	}
	doc := output.Obj{}.Set("model", res.Model).Set("result", verdict).Set("state", state).Set("runs", len(res.Runs))
	var t1Secs, t2Secs, minFree, swap, tok float64
	t1, t2ran := true, false
	score, total, reason := -1, 0, ""
	for _, r := range res.Runs {
		t1 = t1 && r.Tier1Pass
		t1Secs = max(t1Secs, r.Tier1Seconds)
		if r.Tier2Ran {
			t2ran = true
			if score < 0 || r.Tier2Score < score {
				score, total = r.Tier2Score, r.Tier2Total
			}
			t2Secs = max(t2Secs, r.Tier2Seconds)
		}
		if reason == "" {
			reason = r.Reason
		}
		if minFree == 0 || r.MinFreePct < minFree {
			minFree = r.MinFreePct
		}
		swap = max(swap, r.SwapGrowthMB)
		tok += r.GenTokPerSec
	}
	doc = doc.Set("tier1", fmt.Sprintf("%s (%.0fs)", passFail(t1), t1Secs))
	if t2ran {
		doc = doc.Set("tier2", fmt.Sprintf("%d/%d in %.1f min", score, total, t2Secs/60))
	} else {
		doc = doc.Set("tier2", "skipped (tier 1 failed)")
	}
	doc = doc.Set("min_free_pct", int(minFree)).Set("swap_growth_mb", int(swap))
	if len(res.Runs) > 0 {
		doc = doc.Set("tok_per_s", int(tok/float64(len(res.Runs))))
	}
	if reason != "" {
		doc = doc.Set("reason", reason)
	}
	if note != "" {
		doc = doc.Set("note", note)
	}
	return doc.Set("dir", dir)
}

func passFail(ok bool) string {
	if ok {
		return "pass"
	}
	return "fail"
}

// confirmDemotion asks before a previously passed model is demoted. Only a
// person at a terminal is asked; agents and pipes never see a prompt, so the
// demotion just happens (and is reported).
func (a *App) confirmDemotion(name string) bool {
	if !a.deps.StdinTTY || !a.printer.TTY || !a.printer.Human {
		return true
	}
	fmt.Fprintf(a.deps.Err, "%s passed before but failed this time. Demote it to failed and remove it from Pi? [y/N] ", name)
	line, _ := bufio.NewReader(a.deps.In).ReadString('\n')
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(line)), "y")
}

func newBenchSummaryCmd(a *App) *cobra.Command {
	var all bool
	c := &cobra.Command{
		Use:   "summary",
		Short: "Ranked benchmark results for this machine",
		Long: `Rank benchmarked models: passed both tiers first, then Tier 1 only, then
failed; within a group by Tier 2 score, minimum free memory, then Tier 2 time.
By default each registered model shows its latest result; --all shows every
result ever recorded, including removed models.

A * after the llama.cpp build means the result was measured on a different
build than the one installed now.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := a.load(); err != nil {
				return err
			}
			snap, err := a.reg.Load()
			if err != nil {
				return output.Fail(nil, "%v", err)
			}
			results, err := bench.LoadResults(a.paths.BenchmarkDir())
			if err != nil {
				return output.Fail(nil, "%v", err)
			}
			current := bench.BuildOf(a.llamaVersion())
			var keep func(string) bool
			if !all {
				keep = func(model string) bool { _, ok := snap.Models.Find(model); return ok }
			}
			rows := bench.Summarize(results, all, current, keep)
			table := make([]output.Obj, 0, len(rows))
			stale := false
			for _, r := range rows {
				build := r.Build
				if r.OlderBuild {
					build += "*"
					stale = true
				}
				table = append(table, output.Obj{}.
					Set("rank", r.Rank).Set("model", r.Model).Set("result", passFail(r.Passed)).Set("t1", r.Tier1).
					Set("t2", r.Tier2Score).Set("t2_min", fmt.Sprintf("%.1f", r.Tier2Secs/60)).
					Set("min_free_pct", int(r.MinFreePct)).Set("swap_mb", int(r.SwapGrowthM)).
					Set("tok_s", int(r.TokPerSec)).Set("runs", r.Runs).
					Set("llama_cpp", orDash(build)).Set("date", r.Date))
			}
			var help []string
			switch {
			case len(rows) == 0:
				help = append(help, "benchmark a model: pill bench run <name>")
			case stale:
				help = append(help, "* benched on a different llama.cpp build than the installed "+current+"; re-run pill bench run <name> to refresh")
			}
			a.printer.Emit(output.Obj{}.Set("ranking", table).Set("help", help))
			return nil
		},
	}
	c.Flags().BoolVar(&all, "all", false, "show every recorded result, not just the latest per model")
	return c
}

// llamaVersion returns llama-server's version string ("" if unavailable).
func (a *App) llamaVersion() string {
	bin, err := router.LlamaServer()
	if err != nil {
		return ""
	}
	v := llamaVersion(bin)
	if strings.HasPrefix(v, "(") {
		return ""
	}
	return v
}

// olderBuildNotes maps model name to a warning for results measured on a
// different llama.cpp build than the one installed now. A llama.cpp upgrade
// does not invalidate a pass, but it is worth knowing.
func (a *App) olderBuildNotes(snap *snapshot) map[string]string {
	current := bench.BuildOf(a.llamaVersion())
	notes := map[string]string{}
	if current == "" {
		return notes
	}
	for name, st := range snap.Results.Models {
		if st.Latest == nil {
			continue
		}
		if b := bench.BuildOf(st.Latest.Env.LlamaCpp); b != "" && b != current {
			notes[name] = "benched on older llama.cpp (build " + b + ", now " + current + ")"
		}
	}
	return notes
}
