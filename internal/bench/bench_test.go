package bench

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/marcellovictorino/pill/internal/config"
	"github.com/marcellovictorino/pill/internal/router"
	"github.com/marcellovictorino/pill/internal/sysinfo"
	"github.com/marcellovictorino/pill/internal/testutil"
)

func TestMain(m *testing.M) {
	testutil.MaybeRunFake()
	os.Exit(m.Run())
}

func TestAgentPromptIsDeterministicAndSized(t *testing.T) {
	p1, want1 := agentPrompt(300)
	p2, want2 := agentPrompt(300)
	if p1 != p2 || want1 != want2 {
		t.Fatal("the prompt must be deterministic")
	}
	// Roughly 3.5 characters per token: the prompt should be tens of thousands of characters.
	if len(p1) < 20000 || len(p1) > 60000 {
		t.Errorf("prompt is %d characters", len(p1))
	}
	if !strings.Contains(p1, "Question: what does Compute0180(2) return?") || want1 <= 0 {
		t.Errorf("question/answer wrong (want %d)", want1)
	}
}

func TestParsePiOutput(t *testing.T) {
	dir := t.TempDir()
	events := filepath.Join(dir, "events.jsonl")
	body := `{"type":"session"}
{"type":"tool_execution_start","toolName":"bash"}
{"type":"message_end","message":{"role":"assistant","content":[{"type":"toolCall","name":"bash"}]}}
{"type":"tool_execution_start","toolName":"read"}
{"type":"message_end","message":{"role":"user","content":"ignore me"}}
{"type":"message_end","message":{"role":"assistant","content":[{"type":"text","text":" PILL-1234 "}]}}
`
	_ = os.WriteFile(events, []byte(body), 0o644)
	if answer, tools, _ := parsePiOutput(events); answer != "PILL-1234" || tools != 2 {
		t.Errorf("answer=%q tools=%d", answer, tools)
	}

	plain := filepath.Join(dir, "plain.txt")
	_ = os.WriteFile(plain, []byte("just text\n"), 0o644)
	if answer, tools, _ := parsePiOutput(plain); answer != "just text" || tools != 0 {
		t.Errorf("plain: answer=%q tools=%d", answer, tools)
	}
	if answer, tools, _ := parsePiOutput(filepath.Join(dir, "missing")); answer != "" || tools != 0 {
		t.Errorf("missing file: %q %d", answer, tools)
	}
}

func TestParsePiOutputCountsRepeatedCalls(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	call := `{"type":"tool_execution_start","toolName":"bash","args":{"command":"node cli.mjs 1 2"}}` + "\n"
	other := `{"type":"tool_execution_start","toolName":"bash","args":{"command":"ls"}}` + "\n"
	_ = os.WriteFile(path, []byte(call+call+other+call+call+call+other), 0o644)
	if _, tools, repeats := parsePiOutput(path); tools != 7 || repeats != 3 {
		t.Errorf("tools=%d repeats=%d, want 7 and 3", tools, repeats)
	}
}

func TestVerdictNamesAStuckAgent(t *testing.T) {
	rr := config.RunResult{Tier1Pass: true, Tier2Ran: true, Tier2TimedOut: true, Tier2Repeats: 290, Tier2Score: 22, Tier2Total: 22}
	ok, reason := verdict(rr, DefaultThresholds(), Stats{Samples: 3, MinFreePct: 40})
	if ok || !strings.Contains(reason, "repeated the same tool call 290 times") {
		t.Errorf("ok=%v reason=%q", ok, reason)
	}
}

func TestSamplerStats(t *testing.T) {
	sys := &sysinfo.Fake{Samples: []sysinfo.Sample{
		{FreePct: 50, SwapUsedBytes: 1e9, Pressure: 1, GPUUtilPct: 0},
		{FreePct: 22, SwapUsedBytes: 1.5e9, Pressure: 2, GPUUtilPct: 80},
		{FreePct: 35, SwapUsedBytes: 2e9, Pressure: 1, GPUUtilPct: 60},
	}}
	s := StartSampler(context.Background(), sys, time.Hour) // the timer never fires: only the explicit samples count
	st := s.Stop()
	s.Stop() // idempotent
	if st.Samples != 2 {
		t.Fatalf("samples = %d", st.Samples)
	}
	if st.MinFreePct != 22 || st.MaxPressure != 2 || st.SwapGrowthMB != 500 {
		t.Errorf("stats = %+v", st)
	}
	if st.GPUBusyMin != time.Hour.Minutes() { // one busy sample at a one-hour interval
		t.Errorf("gpu busy = %v", st.GPUBusyMin)
	}
}

func TestVerdict(t *testing.T) {
	th := DefaultThresholds()
	ok := Stats{Samples: 2, MinFreePct: 40}
	good := config.RunResult{Tier1Pass: true, Tier2Ran: true, Tier2Score: 22, Tier2Total: 22}
	if pass, why := verdict(good, th, ok); !pass || why != "" {
		t.Errorf("good run: %v %q", pass, why)
	}
	cases := []struct {
		name  string
		rr    config.RunResult
		stats Stats
		want  string
	}{
		{"tier 1 failed", config.RunResult{Tier1Checks: []config.Check{{Name: "agent-sized request within time limit", Detail: "90s"}}}, ok, "tier 1: agent-sized request within time limit: 90s"},
		{"timeout", config.RunResult{Tier1Pass: true, Tier2Ran: true, Tier2TimedOut: true, Tier2Score: 22, Tier2Total: 22}, ok, "hit the 15-minute cap"},
		{"partial score", config.RunResult{Tier1Pass: true, Tier2Ran: true, Tier2Score: 20, Tier2Total: 22}, ok, "score 20/22"},
		{"no total", config.RunResult{Tier1Pass: true, Tier2Ran: true}, ok, "score 0/0"},
		{"memory", good, Stats{Samples: 2, MinFreePct: 7}, "free memory dipped to 7%"},
	}
	for _, c := range cases {
		pass, why := verdict(c.rr, th, c.stats)
		if pass || !strings.Contains(why, c.want) {
			t.Errorf("%s: pass=%v reason=%q, want %q", c.name, pass, why, c.want)
		}
	}
}

func result(model string, when time.Time, build string, passed bool, runs ...config.RunResult) config.Result {
	return config.Result{Model: model, Time: when, Passed: passed, Runs: runs,
		Env: config.Environment{LlamaCpp: "0.6.0 (build " + build + ", commit abc)"}}
}

func run(t1 bool, score int, secs, free float64) config.RunResult {
	return config.RunResult{Tier1Pass: t1, Tier2Ran: t1, Tier2Score: score, Tier2Total: 22, Tier2Seconds: secs, MinFreePct: free, GenTokPerSec: 20, SwapGrowthMB: 10}
}

func TestSummarizeRanking(t *testing.T) {
	day := func(n int) time.Time { return time.Date(2026, 10, n, 12, 0, 0, 0, time.UTC) }
	results := []config.Result{
		result("failed-t1", day(1), "11429", false, run(false, 0, 0, 30)),
		result("t1-only", day(1), "11429", false, run(true, 15, 300, 50)),
		result("slow-pass", day(1), "11429", true, run(true, 22, 600, 40)),
		result("fast-pass", day(1), "11429", true, run(true, 22, 300, 40)),
		result("roomy-pass", day(1), "11429", true, run(true, 22, 900, 60)),
		result("old-then-new", day(1), "11000", false, run(false, 0, 0, 10)),
		result("old-then-new", day(2), "11429", true, run(true, 22, 400, 40)), // latest wins
	}
	rows := Summarize(results, false, "11429", nil)
	var got []string
	for _, r := range rows {
		got = append(got, r.Model)
	}
	want := "roomy-pass,fast-pass,old-then-new,slow-pass,t1-only,failed-t1"
	if strings.Join(got, ",") != want {
		t.Errorf("order = %s, want %s", strings.Join(got, ","), want)
	}
	if rows[0].Rank != 1 || rows[5].Rank != 6 || rows[0].Tier2Score != "22/22" || rows[5].Tier2Score != "-" || rows[5].Tier1 != "fail" {
		t.Errorf("rows = %+v", rows)
	}
	if all := Summarize(results, true, "11429", nil); len(all) != 7 {
		t.Errorf("--all should keep every result, got %d", len(all))
	}
	if only := Summarize(results, false, "11429", func(m string) bool { return m == "t1-only" }); len(only) != 1 {
		t.Errorf("keep filter ignored: %d", len(only))
	}
	if rows := Summarize(results, true, "11500", nil); !rows[0].OlderBuild {
		t.Error("a result from another llama.cpp build must be flagged")
	}
}

func TestLoadResultsReadsResultFiles(t *testing.T) {
	dir := t.TempDir()
	for i, name := range []string{"a", "b"} {
		d := filepath.Join(dir, "2026-"+name)
		_ = os.MkdirAll(d, 0o755)
		data, _ := json.Marshal(result(name, time.Date(2026, 10, 1+i, 0, 0, 0, 0, time.UTC), "1", true))
		_ = os.WriteFile(filepath.Join(d, "result.json"), data, 0o644)
	}
	_ = os.MkdirAll(filepath.Join(dir, "broken"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "broken", "result.json"), []byte("{nope"), 0o644)
	results, err := LoadResults(dir)
	if err != nil || len(results) != 2 || results[0].Model != "a" {
		t.Errorf("results = %+v, %v", results, err)
	}
}

// newRunner wires a Runner to the fake llama-server and fake pi.
func newRunner(t *testing.T) (*Runner, config.Model, string) {
	t.Helper()
	fakes := testutil.InstallFakes(t)
	root := t.TempDir()
	paths := config.Paths{ConfigDir: filepath.Join(root, "config"), Root: root}
	settings := config.Settings{Port: testutil.FreePort(t), IdleSeconds: 900, ModelsDir: filepath.Join(root, "models")}
	if err := os.WriteFile(paths.ModelsIni(), []byte("[*]\n\n[m1]\nmodel = /x.gguf\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rt := router.New(paths, settings)
	if _, err := rt.Start(context.Background(), "h"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = rt.Stop(context.Background()) })

	sys := &sysinfo.Fake{RAM: 24 << 30, Samples: []sysinfo.Sample{{FreePct: 40, Pressure: 1, GPUUtilPct: 50}}}
	r := NewRunner(rt, sys, fakes.Pi, config.Environment{LlamaCpp: "9.9.9 (build 999)", Pill: "test"})
	r.SampleInterval = time.Hour
	r.PromptFunctions = 20
	return r, config.Model{Name: "m1", Ctx: 8192, MaxTokens: 2048}, filepath.Join(root, "bench")
}

func needNode(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node is not installed; Tier 2 verification needs it")
	}
}

func refSolution(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs("testdata/ttt_ref")
	if err != nil {
		t.Fatal(err)
	}
	return "cp '" + abs + "'/*.mjs ."
}

func TestRunnerFullPass(t *testing.T) {
	needNode(t)
	r, model, dir := newRunner(t)
	t.Setenv("PILL_FAKE_PI_TIER2_SCRIPT", refSolution(t))
	res, err := r.Run(context.Background(), Plan{Model: model, Runs: 2, Thresholds: DefaultThresholds(), Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Passed || len(res.Runs) != 2 {
		t.Fatalf("result = %+v", res)
	}
	rr := res.Runs[0]
	if !rr.Tier1Pass || !rr.Tier2Ran || rr.Tier2Score != 22 || rr.Tier2Total != 22 || rr.MinFreePct != 40 || rr.GenTokPerSec != 42.5 {
		t.Errorf("run = %+v", rr)
	}
	for _, f := range []string{"result.json", "run-1/tier1-pi.jsonl", "run-1/tier2-pi.jsonl", "run-2/tier2-workspace/engine.mjs", "run-1/tier2-verify.json", "run-1/pi-agent/models.json", "run-1/samples.json"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Errorf("missing %s", f)
		}
	}
	// The throwaway agent dir holds only the pill provider.
	data, _ := os.ReadFile(filepath.Join(dir, "run-1/pi-agent/models.json"))
	if !strings.Contains(string(data), `"pill"`) || strings.Contains(string(data), "unverified") {
		t.Errorf("bench agent dir:\n%s", data)
	}
}

func TestRunnerFailsTier1WhenPiGivesWrongAnswer(t *testing.T) {
	r, model, dir := newRunner(t)
	t.Setenv("PILL_FAKE_PI_TIER1", "wrong")
	res, err := r.Run(context.Background(), Plan{Model: model, Runs: 1, Thresholds: DefaultThresholds(), Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	rr := res.Runs[0]
	if res.Passed || rr.Tier1Pass || rr.Tier2Ran || !strings.Contains(rr.Reason, "Pi tool-calling turn") {
		t.Errorf("run = %+v", rr)
	}
}

func TestRunnerFailsTier1WithoutToolCall(t *testing.T) {
	r, model, dir := newRunner(t)
	t.Setenv("PILL_FAKE_PI_TIER1", "notool")
	res, _ := r.Run(context.Background(), Plan{Model: model, Runs: 1, Thresholds: DefaultThresholds(), Dir: dir})
	if res.Passed || res.Runs[0].Tier1Pass {
		t.Errorf("an answer without a tool call must not pass Tier 1: %+v", res.Runs[0])
	}
}

func TestRunnerFailsOnLowMemoryAndSlowRequest(t *testing.T) {
	r, model, dir := newRunner(t)
	r.Sys = &sysinfo.Fake{Samples: []sysinfo.Sample{{FreePct: 5, Pressure: 4, GPUUtilPct: -1}}}
	res, _ := r.Run(context.Background(), Plan{Model: model, Runs: 1, Thresholds: DefaultThresholds(), Dir: dir})
	if res.Passed || !strings.Contains(res.Runs[0].Reason, "free memory") {
		t.Errorf("low memory: %+v", res.Runs[0])
	}

	r, model, dir = newRunner(t)
	th := DefaultThresholds()
	th.Tier1MaxSeconds = 0.0000001
	res, _ = r.Run(context.Background(), Plan{Model: model, Runs: 1, Thresholds: th, Dir: dir})
	if res.Passed || !strings.Contains(res.Runs[0].Reason, "within time limit") {
		t.Errorf("slow request: %+v", res.Runs[0])
	}
}

func TestRunnerTier2PartialScoreAndTimeout(t *testing.T) {
	needNode(t)
	r, model, dir := newRunner(t)
	// The fake model writes nothing: the verifier scores 0 of 22.
	res, _ := r.Run(context.Background(), Plan{Model: model, Runs: 1, Thresholds: DefaultThresholds(), Dir: dir})
	rr := res.Runs[0]
	if res.Passed || !rr.Tier1Pass || rr.Tier2Score != 0 || rr.Tier2Total != 22 || !strings.Contains(rr.Reason, "tier 2 score 0/22") {
		t.Errorf("empty workspace: %+v", rr)
	}

	// A model that never finishes is killed at the cap, and the cap fails the run.
	r, model, dir = newRunner(t)
	t.Setenv("PILL_FAKE_PI_TIER2_SLEEP", "30")
	t.Setenv("PILL_FAKE_PI_TIER2_SCRIPT", refSolution(t))
	th := DefaultThresholds()
	th.Tier2MaxMinutes = 0.03 // about two seconds
	start := time.Now()
	res, _ = r.Run(context.Background(), Plan{Model: model, Runs: 1, Thresholds: th, Dir: dir})
	rr = res.Runs[0]
	if res.Passed || !rr.Tier2TimedOut || !strings.Contains(rr.Reason, "cap") {
		t.Errorf("timeout: %+v", rr)
	}
	if time.Since(start) > 25*time.Second {
		t.Error("the Pi process was not killed at the cap")
	}
}

func TestRunnerStopsWhenCancelled(t *testing.T) {
	r, model, dir := newRunner(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := r.Run(ctx, Plan{Model: model, Runs: 1, Thresholds: DefaultThresholds(), Dir: dir}); err == nil {
		t.Error("a cancelled context must stop the benchmark")
	}
}
