// Package bench measures whether a model is good enough to be Pi's default on
// this machine, and ranks the results.
//
// Tier 1 asks: can it serve an agent-sized request quickly without starving
// the machine, and can it drive Pi through a tool call? Tier 2 asks: can it
// build a small, spec-driven program on its own within a time cap, judged by
// a hidden verifier script?
package bench

import (
	"context"
	_ "embed" // enables //go:embed for the task files below
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/marcellovictorino/pill/internal/config"
	"github.com/marcellovictorino/pill/internal/pi"
	"github.com/marcellovictorino/pill/internal/router"
	"github.com/marcellovictorino/pill/internal/sysinfo"
)

// The Tier 2 task and its verifier are compiled into the binary. The
// verifier is written into the workspace only after the agent has finished,
// so the agent never sees what it will be judged on.
var (
	//go:embed tasks/tictactoe.md
	tictactoePrompt string
	//go:embed tasks/verify.mjs
	verifyScript []byte
)

// Runner holds everything a benchmark needs from the outside world.
type Runner struct {
	Router *router.Router
	Sys    sysinfo.Info
	PiBin  string
	Node   string // node executable, "node" by default
	HTTP   *http.Client
	Env    config.Environment

	SampleInterval  time.Duration // how often memory/GPU is sampled (5s)
	PromptFunctions int           // size of the agent-sized prompt (number of functions listed)
	Log             func(format string, args ...any)
}

// Plan describes one `pill bench run`.
type Plan struct {
	Model      config.Model
	Runs       int
	Thresholds config.Thresholds
	Dir        string // result directory, created by Run
}

// DefaultThresholds are the gate values from the design (all are flags).
func DefaultThresholds() config.Thresholds {
	return config.Thresholds{Tier1MaxSeconds: 60, MinFreePct: 10, Tier2MaxMinutes: 15}
}

// NewRunner fills in defaults for the optional fields.
func NewRunner(rt *router.Router, sys sysinfo.Info, piBin string, env config.Environment) *Runner {
	node := os.Getenv("PILL_NODE")
	if node == "" {
		node = "node"
	}
	return &Runner{
		Router: rt, Sys: sys, PiBin: piBin, Node: node, Env: env,
		HTTP:            &http.Client{}, // per-request deadlines come from contexts
		SampleInterval:  5 * time.Second,
		PromptFunctions: 130, // about 8K tokens with Gemma's tokenizer (measured: 300 functions = 18K)
		Log:             func(string, ...any) {},
	}
}

// Run executes the plan and writes result.json (plus transcripts and
// workspaces) under plan.Dir. The model must already be in the router's
// models.ini and the router must be running.
func (r *Runner) Run(ctx context.Context, p Plan) (config.Result, error) {
	res := config.Result{
		ID: filepath.Base(p.Dir), Model: p.Model.Name, Time: time.Now().UTC(),
		Env: r.Env, Thresholds: p.Thresholds,
	}
	if err := os.MkdirAll(p.Dir, 0o755); err != nil {
		return res, err
	}
	for i := 1; i <= p.Runs; i++ {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		r.Log("run %d of %d", i, p.Runs)
		rr, err := r.runOnce(ctx, p, i)
		if err != nil {
			return res, err
		}
		res.Runs = append(res.Runs, rr)
	}
	res.Passed = len(res.Runs) > 0
	for _, rr := range res.Runs {
		res.Passed = res.Passed && rr.Passed // a pass needs every run to pass
	}
	data, _ := json.MarshalIndent(res, "", "  ")
	if err := config.WriteFileAtomic(filepath.Join(p.Dir, "result.json"), append(data, '\n'), 0o644); err != nil {
		return res, err
	}
	return res, nil
}

// runOnce is one cold-start run: unload, Tier 1, and Tier 2 if Tier 1 passed.
func (r *Runner) runOnce(ctx context.Context, p Plan, n int) (config.RunResult, error) {
	var rr config.RunResult
	th := p.Thresholds
	dir := filepath.Join(p.Dir, fmt.Sprintf("run-%d", n))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return rr, err
	}
	// Start cold every run so load time counts, and so a previous run's cached
	// state cannot flatter the model.
	if _, err := r.Router.UnloadAll(ctx); err != nil {
		return rr, fmt.Errorf("unload before run: %w", err)
	}
	agentDir := filepath.Join(dir, "pi-agent")
	if err := pi.WriteBenchAgentDir(agentDir, r.Router.Settings.Port, pi.Model{
		ID: p.Model.Name, Reasoning: p.Model.Reasoning, Ctx: p.Model.Ctx, MaxTokens: p.Model.MaxTokens,
	}); err != nil {
		return rr, err
	}

	sampler := StartSampler(ctx, r.Sys, r.SampleInterval)
	defer sampler.Stop()

	// --- Tier 1a: agent-sized request ---
	r.Log("tier 1: agent-sized request (cold start)")
	chat := r.agentRequest(ctx, p.Model.Name, time.Duration(th.Tier1MaxSeconds*3*float64(time.Second)))
	if ctx.Err() != nil {
		return rr, ctx.Err()
	}
	rr.Tier1Seconds, rr.PromptTokens, rr.GenTokens = chat.Seconds, chat.PromptTokens, chat.GenTokens
	// Speed is informational, measured on the warm model only if it answered.
	decodeErr := ""
	decodeTok := 0
	if chat.Err == nil {
		r.Log("tier 1: decode speed (%d tokens)", decodeTokens)
		var err error
		rr.GenTokPerSec, decodeTok, err = r.decodeSpeed(ctx, p.Model.Name, time.Duration(th.Tier1MaxSeconds*float64(time.Second)))
		if err != nil {
			decodeErr = err.Error()
		}
		if ctx.Err() != nil {
			return rr, ctx.Err()
		}
	}
	reqOK := chat.Err == nil && chat.Seconds <= th.Tier1MaxSeconds
	detail := fmt.Sprintf("%.1fs for ~%d prompt tokens (limit %.0fs)", chat.Seconds, chat.PromptTokens, th.Tier1MaxSeconds)
	if chat.Err != nil {
		detail = chat.Err.Error()
	}
	rr.Tier1Checks = append(rr.Tier1Checks, config.Check{Name: "agent-sized request within time limit", Pass: reqOK, Detail: detail})
	rr.Tier1Checks = append(rr.Tier1Checks, config.Check{Name: "agent-sized request answered correctly (informational)", Pass: chat.Correct, Detail: chat.Reply})
	chatErr := ""
	if chat.Err != nil {
		chatErr = chat.Err.Error()
	}
	saveJSON(filepath.Join(dir, "tier1-chat.json"), map[string]any{
		"seconds": chat.Seconds, "prompt_tokens": chat.PromptTokens, "gen_tokens": chat.GenTokens,
		"reply": chat.Reply, "correct": chat.Correct, "error": chatErr,
		"decode_tok_per_sec": rr.GenTokPerSec, "decode_tokens": decodeTok, "decode_error": decodeErr,
	})

	// --- Tier 1b: a real Pi turn that needs a tool call ---
	token := randomToken()
	ws1 := filepath.Join(dir, "tier1-workspace")
	_ = os.MkdirAll(ws1, 0o755)
	_ = os.WriteFile(filepath.Join(ws1, "facts.txt"), []byte(token+"\n"), 0o644)
	r.Log("tier 1: Pi tool-calling turn")
	turn := r.runPi(ctx, 10*time.Minute, ws1, agentDir, p.Model.Name,
		"Read the file facts.txt with your tools and reply with only the exact code it contains.", dir, "tier1")
	if ctx.Err() != nil {
		return rr, ctx.Err()
	}
	toolOK := turn.Err == nil && !turn.TimedOut && turn.ToolCalls >= 1 && contains(turn.Answer, token)
	toolDetail := fmt.Sprintf("%d tool call(s), answer %q", turn.ToolCalls, truncate(turn.Answer, 60))
	switch {
	case turn.TimedOut:
		toolDetail = "timed out"
	case turn.Err != nil:
		toolDetail = turn.Err.Error()
	}
	rr.Tier1Checks = append(rr.Tier1Checks, config.Check{Name: "Pi tool-calling turn returns the right answer", Pass: toolOK, Detail: toolDetail})

	// Memory is judged over everything Tier 1 did.
	st1 := sampler.Stats()
	memOK := st1.Samples > 0 && st1.MinFreePct >= th.MinFreePct
	memDetail := fmt.Sprintf("minimum %.0f%% free (limit %.0f%%)", st1.MinFreePct, th.MinFreePct)
	if st1.Samples == 0 {
		memDetail = "no memory samples could be taken"
	}
	rr.Tier1Checks = append(rr.Tier1Checks, config.Check{Name: "free memory stayed above the limit", Pass: memOK, Detail: memDetail})
	rr.Tier1Pass = reqOK && toolOK && memOK

	// --- Tier 2: build the embedded task ---
	if rr.Tier1Pass {
		r.Log("tier 2: building the task (cap %.0f min)", th.Tier2MaxMinutes)
		r.tier2(ctx, p, dir, agentDir, &rr)
		if ctx.Err() != nil {
			return rr, ctx.Err()
		}
	}

	stats := sampler.Stop()
	rr.MinFreePct, rr.SwapGrowthMB, rr.GPUBusyMin = stats.MinFreePct, stats.SwapGrowthMB, stats.GPUBusyMin
	saveJSON(filepath.Join(dir, "samples.json"), stats)
	rr.Passed, rr.Reason = verdict(rr, th, stats)
	return rr, nil
}

// tier2 lets Pi build the task, then writes the hidden verifier next to the
// result and runs it with node.
func (r *Runner) tier2(ctx context.Context, p Plan, dir, agentDir string, rr *config.RunResult) {
	rr.Tier2Ran = true
	ws := filepath.Join(dir, "tier2-workspace")
	cap := time.Duration(p.Thresholds.Tier2MaxMinutes * float64(time.Minute))
	run := r.runPi(ctx, cap, ws, agentDir, p.Model.Name, tictactoePrompt, dir, "tier2")
	rr.Tier2Seconds = run.Duration.Seconds()
	rr.Tier2TimedOut = run.TimedOut
	rr.Tier2Repeats = run.Repeats
	if ctx.Err() != nil {
		return
	}

	if err := os.WriteFile(filepath.Join(ws, "verify.mjs"), verifyScript, 0o644); err != nil {
		rr.Tier2Checks = []config.Check{{Name: "verifier", Pass: false, Detail: err.Error()}}
		return
	}
	vctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(vctx, r.Node, "verify.mjs")
	cmd.Dir = ws
	out, err := cmd.Output()
	var report struct {
		Score  int            `json:"score"`
		Total  int            `json:"total"`
		Checks []config.Check `json:"checks"`
	}
	if jerr := json.Unmarshal(lastLine(out), &report); jerr != nil {
		detail := "verifier produced no result"
		if err != nil {
			detail = err.Error()
		}
		rr.Tier2Checks = []config.Check{{Name: "verifier", Pass: false, Detail: detail}}
		return
	}
	rr.Tier2Score, rr.Tier2Total, rr.Tier2Checks = report.Score, report.Total, report.Checks
	_ = os.WriteFile(filepath.Join(dir, "tier2-verify.json"), out, 0o644)
}

// verdict applies the gate: Tier 1 passed, Tier 2 full score within the cap,
// and memory never dipped below the limit during the whole run.
func verdict(rr config.RunResult, th config.Thresholds, stats Stats) (bool, string) {
	switch {
	case !rr.Tier1Pass:
		for _, c := range rr.Tier1Checks {
			if !c.Pass && c.Name != "agent-sized request answered correctly (informational)" {
				return false, "tier 1: " + c.Name + ": " + c.Detail
			}
		}
		return false, "tier 1 failed"
	case rr.Tier2TimedOut && rr.Tier2Repeats >= 10:
		return false, fmt.Sprintf("tier 2 hit the %.0f-minute cap: the agent repeated the same tool call %d times in a row (score %d/%d)",
			th.Tier2MaxMinutes, rr.Tier2Repeats, rr.Tier2Score, rr.Tier2Total)
	case rr.Tier2TimedOut:
		return false, fmt.Sprintf("tier 2 hit the %.0f-minute cap (score %d/%d)", th.Tier2MaxMinutes, rr.Tier2Score, rr.Tier2Total)
	case rr.Tier2Total == 0 || rr.Tier2Score != rr.Tier2Total:
		return false, fmt.Sprintf("tier 2 score %d/%d", rr.Tier2Score, rr.Tier2Total)
	case stats.Samples == 0 || stats.MinFreePct < th.MinFreePct:
		return false, fmt.Sprintf("free memory dipped to %.0f%% (limit %.0f%%)", stats.MinFreePct, th.MinFreePct)
	}
	return true, ""
}
