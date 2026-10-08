package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/marcellovictorino/pill/internal/pi"
	"github.com/marcellovictorino/pill/internal/sysinfo"
)

func needNode(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node is not installed; Tier 2 verification needs it")
	}
}

// solveTier2 makes the fake Pi write a correct tic-tac-toe solution.
func solveTier2(t *testing.T) {
	t.Helper()
	abs, err := filepath.Abs("../internal/bench/testdata/ttt_ref")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("PILL_FAKE_PI_TIER2_SCRIPT", "cp '"+abs+"'/*.mjs .")
}

func TestBenchRunPassesAndPromotes(t *testing.T) {
	needNode(t)
	e := newEnv(t)
	solveTier2(t)
	e.gguf(gemmaFile)
	e.run("add", "gemma4-26b-iq4xs-64k", "--unverified").ok(t)

	r := e.run("bench", "run", "gemma4-26b-iq4xs-64k").ok(t)
	for _, want := range []string{"result: passed", "state: passed", "runs: 1", "tier1: pass", "tier2: 22/22", "min_free_pct: 40", "tok_per_s: 42", "dir: $TMP/pill/benchmark/"} {
		if !strings.Contains(r.out, want) {
			t.Errorf("missing %q in:\n%s", want, r.out)
		}
	}
	if !strings.Contains(r.out, "help[1]: make it the default") {
		t.Errorf("a passed model should suggest becoming the default:\n%s", r.out)
	}

	// Promoted: the "(unverified)" marker is gone from Pi's display name.
	prov, _, _ := pi.ReadProvider(e.piModels())
	if strings.Contains(string(prov), "(unverified)") || !strings.Contains(string(prov), "gemma4-26b-iq4xs-64k") {
		t.Errorf("provider after pass: %s", prov)
	}
	if ls := e.run("ls").ok(t); !strings.Contains(ls.out, "gemma4-26b-iq4xs-64k,passed,") {
		t.Errorf("ls:\n%s", ls.out)
	}
	// The benchmark does not leave the router running a model for no reason.
	e.run("default", "gemma4-26b-iq4xs-64k").ok(t)

	sum := e.run("bench", "summary").ok(t)
	if !strings.Contains(sum.out, "ranking[1]{rank,model,result,t1,t2,t2_min,min_free_pct,swap_mb,tok_s,runs,llama_cpp,date}:") ||
		!strings.Contains(sum.out, "1,gemma4-26b-iq4xs-64k,pass,pass,22/22,") {
		t.Errorf("summary:\n%s", sum.out)
	}
}

func TestBenchRunUsesThrowawayPiDirAndNoSession(t *testing.T) {
	e := newEnv(t)
	e.gguf(gemmaFile)
	e.run("add", "gemma4-26b-iq4xs-64k", "--unverified").ok(t)
	e.run("bench", "run", "gemma4-26b-iq4xs-64k")
	log, _ := os.ReadFile(e.fakes.PiLog)
	text := string(log)
	if !strings.Contains(text, "--model pill/gemma4-26b-iq4xs-64k --thinking off") || !strings.Contains(text, "--no-session") ||
		!strings.Contains(text, "--no-extensions") {
		t.Errorf("bench Pi invocation:\n%s", text)
	}
	if strings.Contains(text, "dir="+filepath.Join(e.dir, "piagent")+" ") {
		t.Error("bench must never run Pi against the user's agent dir")
	}
}

func TestBenchRunFailsTier1AndRemovesFromPi(t *testing.T) {
	e := newEnv(t)
	t.Setenv("PILL_FAKE_PI_TIER1", "wrong")
	e.register()
	r := e.run("bench", "run", "gemma4-26b-iq4xs-64k")
	if r.code != 1 {
		t.Fatalf("a failed gate must exit 1, got %d:\n%s", r.code, r.out)
	}
	for _, want := range []string{"result: failed", "state: failed", "tier2: skipped (tier 1 failed)", "reason: ", "the default model failed"} {
		if !strings.Contains(r.out, want) {
			t.Errorf("missing %q in:\n%s", want, r.out)
		}
	}
	if prov, _, _ := pi.ReadProvider(e.piModels()); prov != nil {
		t.Errorf("a failed model must not stay in Pi: %s", prov)
	}
	ini, _ := os.ReadFile(filepath.Join(e.dir, "pill", "models.ini"))
	if !strings.Contains(string(ini), "[gemma4-26b-iq4xs-64k]") {
		t.Error("a failed model stays in models.ini")
	}
	e.run("default", "gemma4-26b-iq4xs-64k").fails(t, 1, "failed its benchmark")
	e.run().fails(t, 1, "failed its benchmark") // pill itself refuses to launch it

	// The escape hatch: an explicit override puts it back as unverified.
	e.run("add", "gemma4-26b-iq4xs-64k", "--unverified").ok(t)
	if ls := e.run("ls").ok(t); !strings.Contains(ls.out, "gemma4-26b-iq4xs-64k,unverified,") {
		t.Errorf("ls after override:\n%s", ls.out)
	}
}

func TestBenchRunLowMemoryFailsGate(t *testing.T) {
	e := newEnv(t)
	e.sys.Samples = []sysinfo.Sample{{FreePct: 6, Pressure: 4, GPUUtilPct: -1}}
	e.gguf(gemmaFile)
	e.run("add", "gemma4-26b-iq4xs-64k", "--unverified").ok(t)
	r := e.run("bench", "run", "gemma4-26b-iq4xs-64k", "--min-free-pct", "10")
	if r.code != 1 || !strings.Contains(r.out, "free memory stayed above the limit") {
		t.Errorf("exit %d:\n%s", r.code, r.out)
	}
	// Thresholds are flags: the same machine passes Tier 1 with a lower limit.
	r = e.run("bench", "run", "gemma4-26b-iq4xs-64k", "--min-free-pct", "5")
	if strings.Contains(r.out, "free memory stayed above the limit") {
		t.Errorf("flag ignored:\n%s", r.out)
	}
}

func TestBenchRunTier2ScoreAndCap(t *testing.T) {
	needNode(t)
	e := newEnv(t)
	e.gguf(gemmaFile)
	r := e.run("bench", "run", "gemma4-26b-iq4xs-64k") // not registered yet: bench registers it as unverified
	if r.code != 1 || !strings.Contains(r.out, "tier2: 0/22") || !strings.Contains(r.out, "reason: tier 2 score 0/22") {
		t.Errorf("exit %d:\n%s", r.code, r.out)
	}
	// It was registered as unverified for the benchmark but never offered to Pi.
	if prov, _, _ := pi.ReadProvider(e.piModels()); prov != nil {
		t.Errorf("a failed candidate must not reach Pi: %s", prov)
	}

	t.Setenv("PILL_FAKE_PI_TIER2_SLEEP", "30")
	r = e.run("bench", "run", "gemma4-26b-iq4xs-64k", "--tier2-max-minutes", "0.03")
	if !strings.Contains(r.out, "cap") {
		t.Errorf("cap not reported:\n%s", r.out)
	}
}

func TestBenchRunPullsMissingModelFirst(t *testing.T) {
	needNode(t)
	e := newEnv(t)
	hub := gemmaHub(t)
	solveTier2(t)
	r := e.run("bench", "run", "gemma4-26b", "--runs", "2").ok(t)
	if hub.Hits("/resolve/") != 1 || !strings.Contains(r.out, "runs: 2") || !strings.Contains(r.out, "result: passed") {
		t.Errorf("hub hits=%d:\n%s", hub.Hits("/resolve/"), r.out)
	}
}

func TestBenchRunValidatesArguments(t *testing.T) {
	e := newEnv(t)
	e.run("bench", "run", "x", "--runs", "0").fails(t, 2, "--runs 0 is not valid")
	e.run("bench", "run").fails(t, 2, "accepts 1 arg")
	e.run("bench", "run", "no-such-model").fails(t, 1, "unknown model")
}

func TestLaterFailureDemotesPassedModelWithoutPromptInAgentMode(t *testing.T) {
	needNode(t)
	e := newEnv(t)
	solveTier2(t)
	e.gguf(gemmaFile)
	e.run("bench", "run", "gemma4-26b-iq4xs-64k").ok(t)

	t.Setenv("PILL_FAKE_PI_TIER1", "wrong")
	r := e.run("bench", "run", "gemma4-26b-iq4xs-64k")
	if r.code != 1 || !strings.Contains(r.out, "state: failed") {
		t.Errorf("exit %d:\n%s", r.code, r.out)
	}
	if prov, _, _ := pi.ReadProvider(e.piModels()); prov != nil {
		t.Errorf("demoted model still in Pi: %s", prov)
	}
}

func TestSummaryEmptyAndFlags(t *testing.T) {
	e := newEnv(t)
	r := e.run("bench", "summary").ok(t)
	golden(t, "summary_empty", r.out)
	e.run("bench", "summary", "--all").ok(t)
	e.run("bench", "summary", "--bogus").fails(t, 2, "unknown flag")
}

func TestSummaryFlagsOlderLlamaBuild(t *testing.T) {
	needNode(t)
	e := newEnv(t)
	solveTier2(t)
	e.gguf(gemmaFile)
	e.run("bench", "run", "gemma4-26b-iq4xs-64k").ok(t)

	// Pretend llama.cpp was upgraded after the benchmark ran.
	t.Setenv("PILL_FAKE_LLAMA_BUILD", "1000")
	if s := e.run("bench", "summary").ok(t); !strings.Contains(s.out, ",999*,") || !strings.Contains(s.out, "help[1]: * benched on a different llama.cpp build") {
		t.Errorf("summary should flag the older build:\n%s", s.out)
	}
	if ls := e.run("ls").ok(t); !strings.Contains(ls.out, "note") || !strings.Contains(ls.out, "benched on older llama.cpp (build 999, now 1000)") {
		t.Errorf("ls should flag the older build:\n%s", ls.out)
	}
	if d := e.run("doctor").ok(t); !strings.Contains(d.out, "bench-freshness,warn") {
		t.Errorf("doctor should flag the older build:\n%s", d.out)
	}
}
