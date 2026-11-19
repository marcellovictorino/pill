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

	pill := filepath.Join(e.dir, "pill")
	before := map[string]string{}
	for _, p := range []string{filepath.Join(pill, "results.json"), filepath.Join(pill, "config", "models.toml"), e.piModels()} {
		before[p] = readFile(t, p)
	}
	srcDigest := treeDigest(t, piDir)
	bareBench := treeDigest(t, filepath.Join(pill, "benchmark"))

	t.Setenv("PILL_FAKE_PI_TIER1", "wrong") // the tool check fails; that must not demote the model
	r := e.run("bench", "run", "gemma4-26b-iq4xs-64k", "--personal").ok(t)
	for _, want := range []string{"result: personal (informational)", "tool_check: 0/1", "extra_s:", "t2: 22/22", "bare_t2: 22/22", "pill bench summary --personal"} {
		if !strings.Contains(r.out, want) {
			t.Errorf("missing %q in:\n%s", want, r.out)
		}
	}
	if strings.Contains(r.out, "result: passed") || strings.Contains(r.out, "result: failed") {
		t.Errorf("a personal run has no pass or fail:\n%s", r.out)
	}

	for p, was := range before {
		if got := readFile(t, p); got != was {
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
	e.run("bench", "run", "gemma4-26b-iq4xs-64k").ok(t)
	writeUserPiSetup(t, piDir)
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
	for _, want := range []string{"AGENTS.md", "skills/mine/SKILL.md", "extensions/web.ts", "models.json"} {
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
	if !strings.Contains(readFile(t, filepath.Join(copyDir, "models.json")), "gemma4-26b-iq4xs-64k") {
		t.Error("the copy's pill provider must point at the model under test")
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

	e.run("bench", "run", "gemma4-26b-iq4xs-64k").ok(t)
	bareSummary := e.run("bench", "summary").ok(t).out
	e.run("bench", "run", "gemma4-26b-iq4xs-64k", "--personal").ok(t)

	s := e.run("bench", "summary", "--personal").ok(t)
	for _, want := range []string{"personal[1]{model,setup,runs,tool_check,tool_s,extra_s,t2,t2_min,bare_t2,bare_t2_min,date}:", "gemma4-26b-iq4xs-64k,", ",1/1,", ",22/22,", "informational"} {
		if !strings.Contains(s.out, want) {
			t.Errorf("missing %q in:\n%s", want, s.out)
		}
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
