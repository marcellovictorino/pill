package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/marcellovictorino/pill/internal/sysinfo"
	"github.com/marcellovictorino/pill/internal/testutil"
)

// -update rewrites golden files: go test ./cmd -update
var update = flag.Bool("update", false, "rewrite golden files")

func TestMain(m *testing.M) {
	testutil.MaybeRunFake() // acts as fake llama-server / pi / launchctl when invoked under those names
	os.Exit(m.Run())
}

// env is an isolated pill installation: every path pill uses lives under one
// temp dir, so tests can never touch the real ~/.pill, ~/.config or ~/.pi.
type env struct {
	t     *testing.T
	dir   string
	port  int
	fakes testutil.Fakes
	sys   *sysinfo.Fake
}

func newEnv(t *testing.T) *env {
	t.Helper()
	dir := t.TempDir()
	if r, err := filepath.EvalSymlinks(dir); err == nil {
		dir = r // macOS temp dirs sit behind a /var -> /private/var symlink
	}
	e := &env{t: t, dir: dir, port: testutil.FreePort(t), fakes: testutil.InstallFakes(t),
		sys: &sysinfo.Fake{RAM: 24 << 30, RSS: 12 << 30, OS: "27.0", Samples: []sysinfo.Sample{{FreePct: 40, Pressure: 1, GPUUtilPct: -1, GPUMemBytes: 15 << 30}}}}
	t.Setenv("HOME", filepath.Join(dir, "home"))
	t.Setenv("PILL_HOME", filepath.Join(dir, "pill"))
	t.Setenv("PI_CODING_AGENT_DIR", filepath.Join(dir, "piagent"))
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("PILL_PORT", strconv.Itoa(e.port))
	t.Setenv("PILL_LAUNCH_AGENTS_DIR", filepath.Join(dir, "LaunchAgents"))
	t.Setenv("NO_COLOR", "1")
	t.Cleanup(func() { e.run("stop", "--all") })
	return e
}

func (e *env) modelsDir() string { return filepath.Join(e.dir, "pill", "models") }
func (e *env) piModels() string  { return filepath.Join(e.dir, "piagent", "models.json") }

// gguf creates a fake model file (a few bytes are enough: the fake router never reads it).
func (e *env) gguf(name string) {
	e.t.Helper()
	if err := os.MkdirAll(e.modelsDir(), 0o755); err != nil {
		e.t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(e.modelsDir(), name), []byte("GGUF-fake-"+name), 0o644); err != nil {
		e.t.Fatal(err)
	}
}

const gemmaFile = "gemma-4-26B-A4B-it-UD-IQ4_XS.gguf"

// result is the outcome of one pill invocation.
type result struct {
	out, err string
	code     int
}

func (e *env) invoke(tty bool, args ...string) result {
	e.t.Helper()
	var out, errOut bytes.Buffer
	deps := Deps{Out: &out, Err: &errOut, StdoutTTY: tty, Sys: e.sys}
	code := Run(context.Background(), deps, args)
	return result{out: e.norm(out.String()), err: e.norm(errOut.String()), code: code}
}

// run invokes pill as an agent would: stdout is not a terminal, so output is TOON.
func (e *env) run(args ...string) result { return e.invoke(false, args...) }

// norm replaces machine-specific values so golden files are portable.
func (e *env) norm(s string) string {
	s = strings.ReplaceAll(s, e.fakes.Dir, "$FAKES") // fakes live in their own temp dir
	s = strings.ReplaceAll(s, e.dir, "$TMP")
	return strings.ReplaceAll(s, strconv.Itoa(e.port), "$PORT")
}

func (r result) ok(t *testing.T) result {
	t.Helper()
	if r.code != 0 {
		t.Fatalf("exit code %d\nstdout:\n%s\nstderr:\n%s", r.code, r.out, r.err)
	}
	return r
}

func (r result) fails(t *testing.T, code int, contains string) result {
	t.Helper()
	if r.code != code || !strings.Contains(r.out+r.err, contains) {
		t.Fatalf("want exit %d containing %q, got exit %d\nstdout:\n%s\nstderr:\n%s", code, contains, r.code, r.out, r.err)
	}
	return r
}

func golden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name+".golden")
	if *update {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run `go test ./cmd -update` to create it)", err)
	}
	if got != string(want) {
		t.Errorf("%s differs from golden (run `go test ./cmd -update` to accept)\n--- got ---\n%s--- want ---\n%s", name, got, want)
	}
}

// get fetches a path from the fake router.
func (e *env) get(path string) string {
	e.t.Helper()
	resp, err := http.Get("http://127.0.0.1:" + strconv.Itoa(e.port) + path)
	if err != nil {
		e.t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(resp.Body)
	return buf.String()
}

// chat sends a chat completion to the fake router, which marks the model loaded.
func (e *env) chat(model string) {
	e.t.Helper()
	body, _ := json.Marshal(map[string]any{"model": model})
	resp, err := http.Post("http://127.0.0.1:"+strconv.Itoa(e.port)+"/v1/chat/completions", "application/json", bytes.NewReader(body))
	if err != nil {
		e.t.Fatal(err)
	}
	_ = resp.Body.Close()
}

// register sets up the usual starting point: the Gemma GGUF present, registered and default.
func (e *env) register() {
	e.t.Helper()
	e.gguf(gemmaFile)
	e.run("add", "gemma4-26b-iq4xs-64k", "--unverified").ok(e.t)
	e.run("default", "gemma4-26b-iq4xs-64k").ok(e.t)
}
