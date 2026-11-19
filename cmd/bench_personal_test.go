package cmd

import (
	"crypto/sha256"
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/marcellovictorino/pill/internal/config"
	"github.com/marcellovictorino/pill/internal/pi"
)

// treeDigest fingerprints every file under dir (names and contents), so a
// test can prove a folder was only read.
func treeDigest(t *testing.T, dir string) string {
	t.Helper()
	var parts []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			data, _ := os.ReadFile(p)
			parts = append(parts, p+"\x00"+string(data))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(parts)
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x01")))
	return string(sum[:])
}

// writeUserPiSetup adds what a person's own Pi folder holds beyond pill's
// provider: skills, AGENTS.md, an extension, history and the Hindsight add-on.
func writeUserPiSetup(t *testing.T, piDir string) {
	t.Helper()
	files := map[string]string{
		"AGENTS.md":                     "my rules\n",
		"skills/mine/SKILL.md":          "---\nname: mine\n---\n",
		"extensions/web.ts":             "export default {}\n",
		"extensions/hindsight/index.ts": "memory\n",
		"sessions/old.jsonl":            "history\n",
		"settings.json.bak":             "{}",
		"web-search-cache/q.json":       "{}",
		"settings.json":                 `{"theme":"dark","extensions":["extensions/web.ts","extensions/hindsight/index.ts"]}`,
	}
	for name, body := range files {
		p := filepath.Join(piDir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// A personal run is informational (captain: kept apart from the pass or fail
// record): even a failing Tier 1 tool check cannot touch results.json, the
// model's state or Pi's model list, and the record lands in personal/results.json.
func TestPersonalRunNeverTouchesTheGate(t *testing.T) {
	needNode(t)
	e := newEnv(t)
	solveTier2(t)
	e.gguf(gemmaFile)
	e.run("bench", "run", "gemma4-26b-iq4xs-64k").ok(t) // bare baseline: passed, in Pi
	piDir := filepath.Join(e.dir, "piagent")
	writeUserPiSetup(t, piDir)

	// Pi's list is stale (the passed model is missing from it): a personal run must
	// not "repair" it, because it may not change what Pi lists.
	if err := os.Remove(e.piModels()); err != nil {
		t.Fatal(err)
	}
	pill := filepath.Join(e.dir, "pill")
	before := map[string]string{}
	for _, p := range []string{filepath.Join(pill, "results.json"), filepath.Join(pill, "config", "models.toml"), e.piModels()} {
		data, _ := os.ReadFile(p) // Pi's file is absent here
		before[p] = string(data)
	}
	srcDigest := treeDigest(t, piDir)
	bareBench := treeDigest(t, filepath.Join(pill, "benchmark"))

	t.Setenv("PILL_FAKE_PI_TIER1", "wrong") // the tool check fails; that must not demote the model
	r := e.run("bench", "run", "gemma4-26b-iq4xs-64k", "--personal").ok(t)
	for _, want := range []string{"result: personal (informational)", "tool_check: 0/1", "extra_s:", "pill bench summary --personal"} {
		if !strings.Contains(r.out, want) {
			t.Errorf("missing %q in:\n%s", want, r.out)
		}
	}
	// Tier 2 ran in the personal run although its tool check failed (informational).
	for _, want := range []string{"\nt2: 22/22\n", "\nbare_t2: 22/22\n"} {
		if !strings.Contains("\n"+r.out+"\n", want) {
			t.Errorf("missing line %q in:\n%s", strings.TrimSpace(want), r.out)
		}
	}
	if strings.Contains(r.out, "result: passed") || strings.Contains(r.out, "result: failed") {
		t.Errorf("a personal run has no pass or fail:\n%s", r.out)
	}

	for p, was := range before {
		if got, _ := os.ReadFile(p); string(got) != was {
			t.Errorf("%s changed during a personal run:\n%s", p, got)
		}
	}
	if treeDigest(t, piDir) != srcDigest {
		t.Error("the user's Pi folder was written to")
	}
	if treeDigest(t, filepath.Join(pill, "benchmark")) != bareBench {
		t.Error("the bare benchmark folder changed")
	}
	if ls := e.run("ls").ok(t); !strings.Contains(ls.out, "gemma4-26b-iq4xs-64k,passed,") {
		t.Errorf("the model must stay passed:\n%s", ls.out)
	}

	// The per-run record is marked too: copied on its own it must still trip the CI guard.
	raw, _ := filepath.Glob(filepath.Join(pill, "personal", "benchmark", "*", "result.json"))
	if len(raw) != 1 || !strings.Contains(readFile(t, raw[0]), `"personal": true`) {
		t.Errorf("the run's own result.json must carry the personal marker: %v", raw)
	}
	var recs []config.PersonalResult
	if err := json.Unmarshal([]byte(readFile(t, filepath.Join(pill, "personal", "results.json"))), &recs); err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 || !recs[0].Personal || recs[0].SetupHash == "" || recs[0].Passed {
		t.Errorf("personal record: %+v", recs)
	}
}

// Captain: the same tasks with the full setup as used day to day, Hindsight
// left out, online. The bare run keeps its three isolation flags and --offline.
func TestPersonalRunUsesCopyOfSetupWithoutIsolationFlags(t *testing.T) {
	needNode(t)
	e := newEnv(t)
	solveTier2(t)
	e.gguf(gemmaFile)
	piDir := filepath.Join(e.dir, "piagent")
	// The bare run fails Tier 1, so Pi's own models.json does not list the model.
	t.Setenv("PILL_FAKE_PI_TIER1", "wrong")
	e.run("bench", "run", "gemma4-26b-iq4xs-64k").fails(t, 1, "state: failed")
	t.Setenv("PILL_FAKE_PI_TIER1", "")
	writeUserPiSetup(t, piDir)
	liveModels, _ := os.ReadFile(e.piModels()) // may not exist: nothing is offered to Pi
	if err := os.Symlink(filepath.Join(piDir, "missing"), filepath.Join(piDir, "dangling")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(e.fakes.PiLog, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	e.run("bench", "run", "gemma4-26b-iq4xs-64k", "--personal").ok(t)
	personalLog := readFile(t, e.fakes.PiLog)
	for _, banned := range []string{"--offline", "--no-extensions", "--no-skills", "--no-context-files"} {
		if strings.Contains(personalLog, banned) {
			t.Errorf("a personal run must not pass %s:\n%s", banned, personalLog)
		}
	}
	if !strings.Contains(personalLog, "--no-session") || !strings.Contains(personalLog, "/personal/benchmark/") {
		t.Errorf("personal Pi runs in its own folder, without a session:\n%s", personalLog)
	}

	matches, _ := filepath.Glob(filepath.Join(e.dir, "pill", "personal", "benchmark", "*", "run-1", "pi-agent"))
	if len(matches) != 1 {
		t.Fatalf("personal agent folder: %v", matches)
	}
	copyDir := matches[0]
	for _, want := range []string{"AGENTS.md", "skills/mine/SKILL.md", "extensions/web.ts"} {
		if _, err := os.Stat(filepath.Join(copyDir, want)); err != nil {
			t.Errorf("the copy should keep %s", want)
		}
	}
	for _, gone := range []string{"sessions", "web-search-cache", "settings.json.bak", "extensions/hindsight", "dangling"} {
		if _, err := os.Lstat(filepath.Join(copyDir, gone)); err == nil {
			t.Errorf("the copy must leave out %s", gone)
		}
	}
	if s := readFile(t, filepath.Join(copyDir, "settings.json")); strings.Contains(strings.ToLower(s), "hindsight") || !strings.Contains(s, "extensions/web.ts") {
		t.Errorf("settings in the copy: %s", s)
	}
	prov, _, err := pi.ReadProvider(filepath.Join(copyDir, "models.json"))
	if err != nil || !strings.Contains(string(prov), "gemma4-26b-iq4xs-64k") {
		t.Errorf("the copy's pill provider must expose the model under test: %s %v", prov, err)
	}
	// Pi's own list is not touched: the failed model stays out of it.
	if got, _ := os.ReadFile(e.piModels()); string(got) != string(liveModels) {
		t.Errorf("a personal run changed Pi's own models.json:\n%s", got)
	}
}

func TestPersonalRunNeedsARegisteredModel(t *testing.T) {
	e := newEnv(t)
	e.run("bench", "run", "no-such-model", "--personal").fails(t, 1, "not registered")
	if _, err := os.Stat(filepath.Join(e.dir, "pill", "personal", "results.json")); err == nil {
		t.Error("nothing should be recorded")
	}
}

// Captain: `pill bench summary --personal` shows both. The ranking itself stays
// the pass or fail record and does not change.
func TestSummaryPersonalShowsBareBeside(t *testing.T) {
	needNode(t)
	e := newEnv(t)
	solveTier2(t)
	e.gguf(gemmaFile)

	empty := e.run("bench", "summary", "--personal").ok(t)
	if !strings.Contains(empty.out, "personal[0]:") || !strings.Contains(empty.out, "pill bench run <name> --personal") {
		t.Errorf("empty state:\n%s", empty.out)
	}

	e.run("bench", "run", "gemma4-26b-iq4xs-64k").ok(t) // the bare run solves Tier 2: 22/22
	bareSummary := e.run("bench", "summary").ok(t).out
	t.Setenv("PILL_FAKE_PI_TIER2_SCRIPT", "true") // the personal run does not: 0/22
	e.run("bench", "run", "gemma4-26b-iq4xs-64k", "--personal").ok(t)

	s := e.run("bench", "summary", "--personal").ok(t)
	if !strings.Contains(s.out, "personal[1]{model,setup,runs,tool_check,tool_s,extra_s,t2,t2_min,bare_t2,bare_t2_min,date}:") ||
		!strings.Contains(s.out, "informational") {
		t.Errorf("header or help missing:\n%s", s.out)
	}
	var row []string
	for _, line := range strings.Split(s.out, "\n") {
		if f := strings.Split(strings.TrimSpace(line), ","); f[0] == "gemma4-26b-iq4xs-64k" && len(f) == 11 {
			row = f
		}
	}
	if row == nil {
		t.Fatalf("no personal row:\n%s", s.out)
	}
	if row[3] != "1/1" || row[6] != "0/22" || row[8] != "22/22" {
		t.Errorf("tool_check, t2 and bare_t2 should be 1/1, 0/22 and 22/22: %v", row)
	}
	if strings.Contains(s.out, ",pass,") || strings.Contains(s.out, ",fail,") {
		t.Errorf("no pass or fail in personal rows:\n%s", s.out)
	}
	if again := e.run("bench", "summary").ok(t).out; again != bareSummary {
		t.Errorf("the bare ranking changed after a personal run:\n%s\nvs\n%s", bareSummary, again)
	}
}

// Personal results must never sit where a commit could pick them up.
func TestPersonalRunRefusesAGitWorkTree(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	e := newEnv(t)
	e.gguf(gemmaFile)
	e.run("add", "gemma4-26b-iq4xs-64k", "--unverified").ok(t)
	if out, err := exec.Command("git", "-C", e.dir, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	e.run("bench", "run", "gemma4-26b-iq4xs-64k", "--personal").fails(t, 1, "inside a git work tree")
	if _, err := os.Stat(filepath.Join(e.dir, "pill", "personal")); err == nil {
		t.Error("nothing may be written inside the work tree")
	}
}
