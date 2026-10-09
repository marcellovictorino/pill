package cmd

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/marcellovictorino/pill/internal/config"
	"github.com/marcellovictorino/pill/internal/router"
	"github.com/marcellovictorino/pill/internal/testutil"
)

// unhealthy makes the fake router answer /health with 503 until the returned
// function is called, standing in for a llama-server that is stuck or still
// loading. It must be set before the router starts so the router inherits it.
func (e *env) unhealthy(t *testing.T) (on func()) {
	t.Helper()
	flag := filepath.Join(e.dir, "unhealthy")
	t.Setenv("PILL_FAKE_UNHEALTHY", flag)
	return func() {
		if err := os.WriteFile(flag, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestServiceFollowsAPortChange(t *testing.T) {
	e := newEnv(t)
	e.register()
	e.run("service", "install").ok(t)
	oldPort := e.port

	// Same installed LaunchAgent, new port: serve must refresh the plist
	// instead of waiting on a port the saved job never opens.
	newPort := testutil.FreePort(t)
	t.Setenv("PILL_PORT", strconv.Itoa(newPort))
	// Cleanups run last-in first-out: newEnv's stop would run after PILL_PORT
	// is restored and miss the router on the new port, so stop it here first.
	t.Cleanup(func() { e.run("stop", "--all") })
	e.run("serve").ok(t)

	plist, err := os.ReadFile(e.plist())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(plist), "<string>"+strconv.Itoa(newPort)+"</string>") || strings.Contains(string(plist), "<string>"+strconv.Itoa(oldPort)+"</string>") {
		t.Errorf("plist still has the old port:\n%s", plist)
	}
	e.port = newPort
	if ps := e.run("ps").ok(t); !strings.Contains(ps.out, "mode: service") {
		t.Errorf("ps on the new port:\n%s", ps.out)
	}
	if portAnswers(oldPort) {
		t.Error("the router on the old port should be gone")
	}
}

func TestDoctorWarnsWhenTheServiceUsesOldSettings(t *testing.T) {
	e := newEnv(t)
	e.register()
	e.run("service", "install").ok(t)
	t.Setenv("PILL_PORT", strconv.Itoa(testutil.FreePort(t)))
	d := e.run("doctor")
	if !strings.Contains(d.out, "service,warn,\"LaunchAgent was installed with different settings (port, paths) than the current ones\"") {
		t.Errorf("doctor:\n%s", d.out)
	}
}

func TestServiceRecordsItsIniFingerprint(t *testing.T) {
	e := newEnv(t)
	e.register()
	e.run("service", "install").ok(t)
	state, err := os.ReadFile(filepath.Join(e.dir, "pill", "run", "router.json"))
	if err != nil || !strings.Contains(string(state), `"ini_hash":"`) || strings.Contains(string(state), `"ini_hash":""`) {
		t.Fatalf("router.json = %q, %v", state, err)
	}
	// A second serve with nothing changed leaves the running job alone.
	before := strings.Count(e.launchLog(), "bootstrap")
	e.run("serve").ok(t)
	if after := strings.Count(e.launchLog(), "bootstrap"); after != before {
		t.Errorf("serve restarted an up-to-date service (%d -> %d bootstraps)", before, after)
	}
}

func TestServiceRestartsWhenAModelIsRemoved(t *testing.T) {
	e := newEnv(t)
	e.register()
	e.gguf("other-Q4_K_M.gguf")
	e.run("add", "other-Q4_K_M.gguf", "--unverified", "--ctx", "8192").ok(t)
	e.run("service", "install").ok(t)
	if m := e.get("/v1/models"); !strings.Contains(m, "other") {
		t.Fatalf("router should serve the added model:\n%s", m)
	}
	// A router launchd restarted on its own has no fingerprint recorded; the
	// removed id must still be noticed from the model list.
	if err := os.Remove(filepath.Join(e.dir, "pill", "run", "router.json")); err != nil {
		t.Fatal(err)
	}
	e.run("rm", "other-q4-k-m-8k").ok(t)
	if m := e.get("/v1/models"); strings.Contains(m, "other") {
		t.Errorf("the removed model is still offered:\n%s", m)
	}
}

func TestStopAllStopsAnUnhealthyDetachedRouter(t *testing.T) {
	e := newEnv(t)
	e.register()
	hang := e.unhealthy(t)
	e.run("serve").ok(t)
	hang()
	if portAnswers(e.port) {
		t.Fatal("the fake should be unhealthy now")
	}
	// Plain stop has nothing to unload and says so; --all ends the process.
	if r := e.run("stop").ok(t); !strings.Contains(r.out, "router: down") {
		t.Errorf("stop:\n%s", r.out)
	}
	r := e.run("stop", "--all").ok(t)
	if !strings.Contains(r.out, "router: stopped") {
		t.Errorf("stop --all:\n%s", r.out)
	}
	if _, ok := e.router().Find(); ok {
		t.Error("the unhealthy router is still running")
	}
}

func TestStopAllBootsOutAnUnhealthyService(t *testing.T) {
	e := newEnv(t)
	e.register()
	hang := e.unhealthy(t)
	e.run("service", "install").ok(t)
	hang()
	r := e.run("stop", "--all").ok(t)
	if !strings.Contains(r.out, "router: stopped") || !strings.Contains(e.launchLog(), "bootout gui/") {
		t.Errorf("stop --all:\n%s\nlaunchctl:\n%s", r.out, e.launchLog())
	}
	if _, ok := e.router().Find(); ok {
		t.Error("the service router is still running")
	}
}

func TestStopNeverTouchesAForeignServer(t *testing.T) {
	e := newEnv(t)
	e.register()
	other := filepath.Join(e.dir, "other.ini")
	if err := os.WriteFile(other, []byte("[*]\n\n[theirs]\nmodel = /x.gguf\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := execFake(t, e, other)
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	e.chat("theirs") // someone else's model is loaded

	for _, args := range [][]string{{"stop"}, {"stop", "--all"}} {
		e.run(args...).fails(t, 1, "pill will not touch it")
	}
	// A benchmark unloads models to measure a cold start; it must not do that
	// to a server pill did not start.
	e.run("bench", "run", "gemma4-26b-iq4xs-64k").fails(t, 1, "pill will not touch it")
	if m := e.get("/v1/models"); !strings.Contains(m, `"value":"loaded"`) {
		t.Errorf("the foreign server's model was unloaded:\n%s", m)
	}
	if cmd.ProcessState != nil {
		t.Error("the foreign server was stopped")
	}
}

// router returns a pill router for the env, to ask the same questions pill asks
// (is anything still serving the port?).
func (e *env) router() *router.Router {
	return router.New(config.Paths{Root: filepath.Join(e.dir, "pill")}, config.Settings{Port: e.port})
}

func TestStopAllOnAnotherPortLeavesTheServiceAlone(t *testing.T) {
	e := newEnv(t)
	e.register()
	e.run("service", "install").ok(t)
	servicePort := e.port

	t.Setenv("PILL_PORT", strconv.Itoa(testutil.FreePort(t)))
	r := e.run("stop", "--all").ok(t)
	if strings.Contains(e.launchLog(), "bootout") {
		t.Errorf("stop --all on an unused port booted out the service on %d:\n%s\n%s", servicePort, r.out, e.launchLog())
	}
	if !portAnswers(servicePort) {
		t.Error("the service router on the other port was stopped")
	}
}

func TestServiceChangeIsRefusedWhileItsRouterHasAModelLoaded(t *testing.T) {
	e := newEnv(t)
	e.register()
	e.run("service", "install").ok(t)
	e.chat("gemma4-26b-iq4xs-64k")

	t.Setenv("PILL_PORT", strconv.Itoa(testutil.FreePort(t)))
	e.run("serve").fails(t, 1, "not replacing it while it may be in use")
	if !portAnswers(e.port) {
		t.Error("the busy service router must keep running")
	}
	if strings.Count(e.launchLog(), "bootstrap") != 1 {
		t.Errorf("the job must not be reloaded:\n%s", e.launchLog())
	}
}

func TestServerIdleTimeChangeRefreshesAHealthyService(t *testing.T) {
	e := newEnv(t)
	e.register()
	e.run("service", "install").ok(t)
	cfg := filepath.Join(e.dir, "pill", "config", "config.toml")
	data, _ := os.ReadFile(cfg)
	if err := os.WriteFile(cfg, append(data, []byte("\nidle_seconds = 123\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	e.run("serve").ok(t)
	plist, _ := os.ReadFile(e.plist())
	if !strings.Contains(string(plist), "<string>123</string>") {
		t.Errorf("the plist still has the old idle time:\n%s", plist)
	}
}

func TestOwnershipIsCheckedEvenWhenHealthFails(t *testing.T) {
	e := newEnv(t)
	e.register()
	hang := e.unhealthy(t)
	other := filepath.Join(e.dir, "other.ini")
	if err := os.WriteFile(other, []byte("[*]\n\n[theirs]\nmodel = /x.gguf\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := execFake(t, e, other)
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	hang() // the foreign server stops answering /health but still holds the port
	e.run("stop", "--all").fails(t, 1, "pill will not touch it")
	e.run("bench", "run", "gemma4-26b-iq4xs-64k").fails(t, 1, "pill will not touch it")
}

func TestServiceReinstallIsRefusedWhileAModelIsLoaded(t *testing.T) {
	e := newEnv(t)
	e.register()
	e.run("service", "install").ok(t)
	e.chat("gemma4-26b-iq4xs-64k")
	e.run("service", "install").fails(t, 1, "not replacing it while it may be in use")
	if !portAnswers(e.port) || strings.Count(e.launchLog(), "bootstrap") != 1 {
		t.Errorf("the busy service must be left alone:\n%s", e.launchLog())
	}
}

// A service router that is there but not answering cannot be shown to be idle,
// so changing the settings must not replace it.
func TestServiceChangeIsRefusedWhenItsRouterDoesNotAnswer(t *testing.T) {
	e := newEnv(t)
	e.register()
	hang := e.unhealthy(t)
	e.run("service", "install").ok(t)
	hang()
	t.Setenv("PILL_PORT", strconv.Itoa(testutil.FreePort(t)))
	e.run("serve").fails(t, 1, "cannot tell whether it is in use")
	if strings.Count(e.launchLog(), "bootstrap") != 1 || strings.Contains(e.launchLog(), "bootout") {
		t.Errorf("the service must not be touched:\n%s", e.launchLog())
	}
}

// Installing onto a port held by a server that does not answer must be refused
// before the old service is stopped or its plist rewritten.
func TestServiceInstallKeepsTheOldServiceWhenTheNewPortIsOccupied(t *testing.T) {
	e := newEnv(t)
	e.register()
	e.run("service", "install").ok(t)
	oldPlist := readFile(t, e.plist())
	oldPort := e.port

	newPort := testutil.FreePort(t)
	t.Setenv("PILL_PORT", strconv.Itoa(newPort))
	e.port = newPort
	hang := e.unhealthy(t) // set after the service started, so only the new server inherits it
	other := filepath.Join(e.dir, "other.ini")
	if err := os.WriteFile(other, []byte("[*]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := execFake(t, e, other)
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	hang()

	e.run("service", "install").fails(t, 1, "does not answer /health")
	if readFile(t, e.plist()) != oldPlist || strings.Contains(e.launchLog(), "bootout") || !portAnswers(oldPort) {
		t.Errorf("the old service was disturbed:\n%s", e.launchLog())
	}
}

// Changing a model's settings without changing its name must still restart a
// service router: the recorded models.ini fingerprint is what notices it.
func TestServiceRestartsWhenAPresetChangesButTheNameDoesNot(t *testing.T) {
	e := newEnv(t)
	e.register()
	e.run("service", "install").ok(t)
	toml := filepath.Join(e.dir, "pill", "config", "models.toml")
	data := readFile(t, toml)
	if !strings.Contains(data, "temp = 1.0") {
		t.Fatalf("expected a temp setting in models.toml:\n%s", data)
	}
	if err := os.WriteFile(toml, []byte(strings.Replace(data, "temp = 1.0", "temp = 0.5", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	before := strings.Count(e.launchLog(), "bootstrap")
	e.run("serve").ok(t)
	if after := strings.Count(e.launchLog(), "bootstrap"); after != before+1 {
		t.Errorf("a changed preset should restart the service once (%d -> %d):\n%s", before, after, e.launchLog())
	}
}

// Two context sizes of one model are two entries; neither replaces the other.
func TestAddKeepsDistinctEntriesForDifferentContextSizes(t *testing.T) {
	e := newEnv(t)
	e.register()
	e.run("add", "gemma4-26b", "--ctx", "8192", "--unverified").ok(t)
	e.run("add", "gemma4-26b", "--ctx", "9000", "--unverified").ok(t)
	ls := e.run("ls").ok(t).out
	for _, want := range []string{"gemma4-26b-iq4xs-8k", "gemma4-26b-iq4xs-9000tok", "gemma4-26b-iq4xs-64k"} {
		if !strings.Contains(ls, want) {
			t.Errorf("ls lacks %s:\n%s", want, ls)
		}
	}
}
