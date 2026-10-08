package cmd

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/marcellovictorino/pill/internal/pi"
	"github.com/marcellovictorino/pill/internal/version"
)

func TestVersionToonHasNoLogo(t *testing.T) {
	e := newEnv(t)
	r := e.run("version").ok(t)
	if !strings.HasPrefix(r.out, "version: "+version.Version+"\ncommit: ") || strings.Contains(r.out, "'-") {
		t.Errorf("unexpected output:\n%s", r.out)
	}
	if r := e.run("--version").ok(t); !strings.HasPrefix(r.out, "version: "+version.Version) {
		t.Errorf("--version:\n%s", r.out)
	}
}

// unsetenv removes a variable for the duration of the test. t.Setenv first
// registers the restore of the original value; the Unsetenv then removes it.
func unsetenv(t *testing.T, name string) {
	t.Helper()
	t.Setenv(name, "")
	if err := os.Unsetenv(name); err != nil {
		t.Fatal(err)
	}
}

// plainText removes colour escape sequences so a coloured logo can be matched
// by its visible characters.
func plainText(s string) string { return ansiSeq.ReplaceAllString(s, "") }

var ansiSeq = regexp.MustCompile("\x1b\\[[0-9;]*m")

func TestLogoOnlyOnTerminalWithoutAgentEnv(t *testing.T) {
	e := newEnv(t)
	// This test suite itself may run inside an agent session, so clear every
	// variable that marks one before checking the logo appears.
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		if strings.HasPrefix(name, "CLAUDE") || strings.HasPrefix(name, "PI_") || strings.HasPrefix(name, "CODEX_") ||
			strings.HasPrefix(name, "OPENCODE") || name == "AI_AGENT" || name == "CURSOR_AGENT" || name == "GEMINI_CLI" || name == "NO_COLOR" {
			unsetenv(t, name)
		}
	}
	tty := e.invoke(true, "--version").ok(t)
	if !strings.Contains(plainText(tty.out), "(pi|ll)") || !strings.Contains(tty.out, "pill "+version.Version) {
		t.Errorf("terminal --version should show the logo:\n%s", tty.out)
	}
	help := e.invoke(true, "--help").ok(t)
	if !strings.Contains(plainText(help.out), "(pi|ll)") {
		t.Errorf("terminal --help should show the logo:\n%s", help.out)
	}
	piped := e.invoke(false, "--help").ok(t)
	if strings.Contains(plainText(piped.out), "(pi|ll)") {
		t.Errorf("piped --help must not show the logo:\n%s", piped.out)
	}

	t.Setenv("CLAUDECODE", "1")
	if r := e.invoke(true, "--version").ok(t); strings.Contains(plainText(r.out), "(pi|ll)") {
		t.Errorf("agent sessions must not get the logo:\n%s", r.out)
	}
	unsetenv(t, "CLAUDECODE")

	t.Setenv("NO_COLOR", "1")
	if r := e.invoke(true, "--version").ok(t); strings.Contains(plainText(r.out), "(pi|ll)") {
		t.Errorf("NO_COLOR must hide the logo:\n%s", r.out)
	}
}

func TestAddRequiresUnverifiedFlag(t *testing.T) {
	e := newEnv(t)
	e.gguf(gemmaFile)
	r := e.run("add", "gemma4-26b-iq4xs-64k").fails(t, 2, "--unverified")
	if !strings.Contains(r.out, "help[1]:") {
		t.Errorf("expected help lines:\n%s", r.out)
	}
	if _, err := os.Stat(filepath.Join(e.dir, "pill", "models.ini")); err == nil {
		t.Error("a refused add must not write anything")
	}
}

func TestAddRegistersEverywhere(t *testing.T) {
	e := newEnv(t)
	e.gguf(gemmaFile)
	r := e.run("add", "gemma4-26b-iq4xs-64k", "--unverified").ok(t)
	golden(t, "add", r.out)

	ini, _ := os.ReadFile(filepath.Join(e.dir, "pill", "models.ini"))
	if !strings.Contains(string(ini), "[gemma4-26b-iq4xs-64k]") || !strings.Contains(string(ini), "c = 65536") ||
		!strings.Contains(string(ini), filepath.Join(e.modelsDir(), gemmaFile)) {
		t.Errorf("models.ini:\n%s", ini)
	}
	toml, _ := os.ReadFile(filepath.Join(e.dir, "pill", "config", "models.toml"))
	if !strings.Contains(string(toml), `name = "gemma4-26b-iq4xs-64k"`) {
		t.Errorf("models.toml:\n%s", toml)
	}
	prov, _, err := pi.ReadProvider(e.piModels())
	if err != nil || prov == nil || len(pi.ProviderModelIDs(prov)) != 1 {
		t.Fatalf("pi provider: %s %v", prov, err)
	}
	if !strings.Contains(string(prov), "(unverified)") || !strings.Contains(string(prov), `"contextWindow": 65536`) {
		t.Errorf("provider: %s", prov)
	}

	// Idempotent: a second add changes nothing and says so.
	again := e.run("add", "gemma4-26b-iq4xs-64k", "--unverified").ok(t)
	if !strings.Contains(again.out, "status: already registered") {
		t.Errorf("second add:\n%s", again.out)
	}
	// The short catalog name resolves to the same entry.
	short := e.run("add", "gemma4-26b", "--unverified").ok(t)
	if !strings.Contains(short.out, "model: gemma4-26b-iq4xs-64k") {
		t.Errorf("short name:\n%s", short.out)
	}
}

func TestAddCustomContextCreatesSecondEntryOnSameFile(t *testing.T) {
	e := newEnv(t)
	e.gguf(gemmaFile)
	e.run("add", "gemma4-26b", "--unverified", "--ctx", "32768").ok(t)
	e.run("add", "gemma4-26b", "--unverified").ok(t)
	ini, _ := os.ReadFile(filepath.Join(e.dir, "pill", "models.ini"))
	if !strings.Contains(string(ini), "[gemma4-26b-iq4xs-32k]") || !strings.Contains(string(ini), "[gemma4-26b-iq4xs-64k]") {
		t.Errorf("expected two sections on one file:\n%s", ini)
	}
}

func TestAddWithoutFileExplainsHowToGetIt(t *testing.T) {
	e := newEnv(t)
	e.run("add", "gemma4-26b-iq4xs-64k", "--unverified").fails(t, 1, "is not in")
	e.run("add", "no-such-model", "--unverified").fails(t, 1, "unknown model")
}

func TestAddAcceptsLocalGGUFFile(t *testing.T) {
	e := newEnv(t)
	e.gguf("tiny-Q4_K_M.gguf")
	r := e.run("add", "tiny-Q4_K_M.gguf", "--unverified").ok(t)
	if !strings.Contains(r.out, "model: tiny-q4-k-m-32k") || !strings.Contains(r.out, "ctx: 32768") {
		t.Errorf("generic file:\n%s", r.out)
	}
}

func TestDefaultFlow(t *testing.T) {
	e := newEnv(t)
	e.run("default", "gemma4-26b-iq4xs-64k").fails(t, 1, "is not registered")
	e.gguf(gemmaFile)
	e.run("add", "gemma4-26b-iq4xs-64k", "--unverified").ok(t)

	if r := e.run("default").ok(t); !strings.Contains(r.out, "default: none") {
		t.Errorf("no default yet:\n%s", r.out)
	}
	r := e.run("default", "gemma4-26b").ok(t) // short name accepted
	if !strings.Contains(r.out, "default: gemma4-26b-iq4xs-64k") || !strings.Contains(r.out, "status: set") {
		t.Errorf("default:\n%s", r.out)
	}
	if r := e.run("default", "gemma4-26b-iq4xs-64k").ok(t); !strings.Contains(r.out, "status: unchanged") {
		t.Errorf("repeat default:\n%s", r.out)
	}
	cfg, _ := os.ReadFile(filepath.Join(e.dir, "pill", "config", "config.toml"))
	if !strings.Contains(string(cfg), `default = "gemma4-26b-iq4xs-64k"`) {
		t.Errorf("config.toml:\n%s", cfg)
	}
}

func TestServePsStopLifecycle(t *testing.T) {
	e := newEnv(t)
	e.register()

	if r := e.run("ps").ok(t); !strings.Contains(r.out, "router: down") {
		t.Errorf("before serve:\n%s", r.out)
	}
	if r := e.run("stop").ok(t); !strings.Contains(r.out, "router: down") {
		t.Errorf("stop when down must be a no-op:\n%s", r.out)
	}

	if r := e.run("serve").ok(t); !strings.Contains(r.out, "status: started") {
		t.Errorf("serve:\n%s", r.out)
	}
	if r := e.run("serve").ok(t); !strings.Contains(r.out, "status: already running") {
		t.Errorf("second serve must be a no-op:\n%s", r.out)
	}

	idle := e.run("ps").ok(t)
	if !strings.Contains(idle.out, "mode: detached") || !strings.Contains(idle.out, "loaded: none") {
		t.Errorf("idle ps:\n%s", idle.out)
	}

	e.chat("gemma4-26b-iq4xs-64k")
	busy := e.run("ps").ok(t)
	if !strings.Contains(busy.out, "loaded: gemma4-26b-iq4xs-64k") || !strings.Contains(busy.out, "rss: 12.9 GB") || !strings.Contains(busy.out, "gpu_memory: 16.1 GB") {
		t.Errorf("busy ps:\n%s", busy.out)
	}
	golden(t, "ps_loaded", strings.Join(dropLines(busy.out, "pid:"), ""))

	if r := e.run("stop").ok(t); !strings.Contains(r.out, "unloaded[1]: gemma4-26b-iq4xs-64k") || !strings.Contains(r.out, "router: up") {
		t.Errorf("stop:\n%s", r.out)
	}
	if r := e.run("stop").ok(t); !strings.Contains(r.out, "unloaded[0]:") {
		t.Errorf("second stop must be a no-op:\n%s", r.out)
	}
	if r := e.run("stop", "--all").ok(t); !strings.Contains(r.out, "router: stopped") {
		t.Errorf("stop --all:\n%s", r.out)
	}
	if r := e.run("stop", "--all").ok(t); !strings.Contains(r.out, "router: down") {
		t.Errorf("repeat stop --all:\n%s", r.out)
	}
	if r := e.run("ps").ok(t); !strings.Contains(r.out, "router: down") {
		t.Errorf("after stop:\n%s", r.out)
	}
}

// dropLines removes lines starting with prefix (volatile values such as pids).
func dropLines(s, prefix string) []string {
	var keep []string
	for _, l := range strings.SplitAfter(s, "\n") {
		if !strings.HasPrefix(l, prefix) {
			keep = append(keep, l)
		}
	}
	return keep
}

func TestAddRefreshesRunningRouter(t *testing.T) {
	e := newEnv(t)
	e.register()
	e.run("serve").ok(t)
	if body := e.get("/v1/models"); strings.Contains(body, "32k") {
		t.Fatalf("unexpected model before add: %s", body)
	}
	// Adding a model while the router is idle restarts it so the new entry appears.
	e.run("add", "gemma4-26b", "--unverified", "--ctx", "32768").ok(t)
	if body := e.get("/v1/models"); !strings.Contains(body, "gemma4-26b-iq4xs-32k") {
		t.Errorf("router did not pick up the new model: %s", body)
	}
}

func TestPiLaunchExecsPiOnDefaultModel(t *testing.T) {
	e := newEnv(t)
	e.register()

	var gotBin string
	var gotArgs []string
	orig := pi.ExecFunc
	defer func() { pi.ExecFunc = orig }()
	pi.ExecFunc = func(bin string, args []string, env []string) error {
		gotBin, gotArgs = bin, args
		return nil
	}

	// A successful exec never returns in real life; the recorder returns nil,
	// which pill reports as an error, so only the recorded call matters here.
	e.run()
	if gotBin != e.fakes.Pi {
		t.Errorf("exec'd %q, want %q", gotBin, e.fakes.Pi)
	}
	want := "pi --model pill/gemma4-26b-iq4xs-64k --thinking off"
	if strings.Join(gotArgs, " ") != want {
		t.Errorf("args = %v, want %s", gotArgs, want)
	}
	if r := e.run("ps").ok(t); !strings.Contains(r.out, "router: up") {
		t.Errorf("pill must start the router before exec:\n%s", r.out)
	}

	e.run("pi", "--model", "other/x", "fix it")
	if want := "pi --thinking off --model other/x fix it"; strings.Join(gotArgs, " ") != want {
		t.Errorf("pass-through args = %v, want %s", gotArgs, want)
	}
}

func TestPiLaunchWithoutDefault(t *testing.T) {
	e := newEnv(t)
	r := e.run().fails(t, 1, "no default model set")
	if !strings.Contains(r.out, "pill default") {
		t.Errorf("help should name the fix:\n%s", r.out)
	}
	// A default whose file vanished is reported, not launched.
	e.register()
	if err := os.Remove(filepath.Join(e.modelsDir(), gemmaFile)); err != nil {
		t.Fatal(err)
	}
	e.run().fails(t, 1, "is missing")
}

func TestUsageErrorsExitTwoAndListChoices(t *testing.T) {
	e := newEnv(t)
	r := e.run("add", "x", "--bogus").fails(t, 2, "unknown flag: --bogus")
	if !strings.Contains(r.out, "valid flags:") || !strings.Contains(r.out, "--unverified") {
		t.Errorf("should list valid flags:\n%s", r.out)
	}
	r = e.run("frobnicate").fails(t, 2, "unknown command")
	if !strings.Contains(r.out, "valid commands:") || !strings.Contains(r.out, "serve") {
		t.Errorf("should list valid commands:\n%s", r.out)
	}
	e.run("add").fails(t, 2, "accepts 1 arg")
	e.run("--format", "yaml", "ps").fails(t, 2, "unknown --format")
}

func TestFormatHumanRendersTables(t *testing.T) {
	e := newEnv(t)
	r := e.run("--format", "human", "doctor")
	if !strings.Contains(r.out, "CHECK") || !strings.Contains(r.out, "llama-server") {
		t.Errorf("human doctor should render a table:\n%s", r.out)
	}
}

func TestDoctor(t *testing.T) {
	e := newEnv(t)
	e.register()
	// A leftover local provider and a GGUF hiding in the config dir are warnings.
	legacy := `{"providers":{"llama-cpp":{"baseUrl":"http://127.0.0.1:8080/v1"}}}`
	if err := os.MkdirAll(filepath.Dir(e.piModels()), 0o755); err != nil {
		t.Fatal(err)
	}
	prov, _, _ := pi.ReadProvider(e.piModels())
	_ = prov
	data, _ := os.ReadFile(e.piModels())
	merged := strings.Replace(string(data), `"providers": {`, `"providers": {"llama-cpp":{"baseUrl":"http://127.0.0.1:8080/v1"},`, 1)
	if err := os.WriteFile(e.piModels(), []byte(merged), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = legacy
	cfgDir := filepath.Join(e.dir, "pill", "config")
	if err := os.WriteFile(filepath.Join(cfgDir, "oops.gguf"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	r := e.run("doctor").ok(t)
	golden(t, "doctor", r.out)
	if got, _ := os.ReadFile(e.piModels()); !strings.Contains(string(got), "llama-cpp") {
		t.Error("doctor must never remove another provider")
	}
}

func TestDoctorFailsWithoutPrerequisites(t *testing.T) {
	e := newEnv(t)
	t.Setenv("PILL_PI", filepath.Join(e.dir, "missing-pi"))
	t.Setenv("PILL_LLAMA_SERVER", "")
	t.Setenv("PATH", e.dir) // nothing installed here
	r := e.run("doctor")
	if r.code != 1 || !strings.Contains(r.out, "llama-server,fail") {
		t.Errorf("want exit 1 with a failing llama-server check, got %d:\n%s", r.code, r.out)
	}
}

func TestDoctorFlagsForeignPortOwner(t *testing.T) {
	e := newEnv(t)
	e.register()
	// Start the fake under a different models.ini so it is not "ours".
	other := filepath.Join(e.dir, "other.ini")
	_ = os.WriteFile(other, []byte("[*]\n"), 0o644)
	cmd := execFake(t, e, other)
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	r := e.run("doctor")
	if r.code != 1 || !strings.Contains(r.out, "port,fail") {
		t.Errorf("want a failing port check, got %d:\n%s", r.code, r.out)
	}
	e.run("stop", "--all").fails(t, 1, "pill will not touch it")
}
