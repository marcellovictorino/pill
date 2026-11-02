package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/marcellovictorino/pill/internal/pi"
	"github.com/marcellovictorino/pill/internal/service"
)

func (e *env) plist() string {
	return filepath.Join(e.dir, "LaunchAgents", service.Label+".plist")
}

func (e *env) launchLog() string {
	data, _ := os.ReadFile(e.fakes.LaunchLog)
	return string(data)
}

func TestServiceInstallLifecycle(t *testing.T) {
	e := newEnv(t)
	e.register()

	r := e.run("service", "install").ok(t)
	for _, want := range []string{"service: installed", "label: " + service.Label, "router: up", "plist: $TMP"} {
		if !strings.Contains(e.norm(r.out), want) {
			t.Errorf("missing %q in:\n%s", want, r.out)
		}
	}
	plist, err := os.ReadFile(e.plist())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"<key>KeepAlive</key>", "<key>RunAtLoad</key>", e.fakes.LlamaServer, "--models-preset", "server.log"} {
		if !strings.Contains(string(plist), want) {
			t.Errorf("plist lacks %q:\n%s", want, plist)
		}
	}
	if strings.Contains(string(plist), os.Getenv("PATH")) {
		t.Error("the plist must not copy the caller's PATH")
	}
	if !strings.Contains(e.launchLog(), "bootstrap gui/") {
		t.Errorf("launchctl log:\n%s", e.launchLog())
	}
	if ps := e.run("ps").ok(t); !strings.Contains(ps.out, "mode: service") {
		t.Errorf("ps:\n%s", ps.out)
	}
	if d := e.run("doctor").ok(t); !strings.Contains(d.out, "service,ok,LaunchAgent "+service.Label+" is loaded") {
		t.Errorf("doctor:\n%s", d.out)
	}

	// Installing again replaces the job instead of failing.
	e.run("service", "install").ok(t)

	// stop --all boots the job out; a SIGTERM would just be undone by KeepAlive.
	st := e.run("stop", "--all").ok(t)
	if !strings.Contains(st.out, "router: stopped") || !strings.Contains(st.out, "LaunchAgent is unloaded") {
		t.Errorf("stop --all:\n%s", st.out)
	}
	if !strings.Contains(e.launchLog(), "bootout gui/") {
		t.Errorf("launchctl log:\n%s", e.launchLog())
	}
	if ps := e.run("ps").ok(t); !strings.Contains(ps.out, "router: down") {
		t.Errorf("ps after stop:\n%s", ps.out)
	}
	if d := e.run("doctor").ok(t); !strings.Contains(d.out, "service,warn,LaunchAgent is installed but not loaded") {
		t.Errorf("doctor:\n%s", d.out)
	}

	// serve brings it back through launchd rather than a detached process.
	e.run("serve").ok(t)
	if ps := e.run("ps").ok(t); !strings.Contains(ps.out, "mode: service") {
		t.Errorf("ps after serve:\n%s", ps.out)
	}

	u := e.run("service", "uninstall").ok(t)
	if !strings.Contains(u.out, "service: removed") {
		t.Errorf("uninstall:\n%s", u.out)
	}
	if _, err := os.Stat(e.plist()); !os.IsNotExist(err) {
		t.Errorf("plist should be gone: %v", err)
	}
	if ps := e.run("ps").ok(t); !strings.Contains(ps.out, "router: down") {
		t.Errorf("ps after uninstall:\n%s", ps.out)
	}
	if u := e.run("service", "uninstall").ok(t); !strings.Contains(u.out, "not installed") {
		t.Errorf("second uninstall should be a no-op:\n%s", u.out)
	}
}

func TestServiceRouterPicksUpModelChanges(t *testing.T) {
	e := newEnv(t)
	e.register()
	e.run("service", "install").ok(t)

	e.gguf("other-Q4_K_M.gguf")
	e.run("add", "other-Q4_K_M.gguf", "--unverified", "--ctx", "8192").ok(t)
	if models := e.get("/v1/models"); !strings.Contains(models, "other-q4-k-m") && !strings.Contains(models, "other") {
		t.Errorf("service router was not restarted with the new models.ini:\n%s", models)
	}
	if !strings.Contains(e.launchLog(), "bootout") || strings.Count(e.launchLog(), "bootstrap") < 2 {
		t.Errorf("a changed models.ini restarts the job through launchctl:\n%s", e.launchLog())
	}
}

func TestServiceInstallReplacesIdleDetachedRouter(t *testing.T) {
	e := newEnv(t)
	e.register()
	e.run("serve").ok(t)
	if ps := e.run("ps").ok(t); !strings.Contains(ps.out, "mode: detached") {
		t.Fatalf("ps:\n%s", ps.out)
	}
	e.run("service", "install").ok(t)
	if ps := e.run("ps").ok(t); !strings.Contains(ps.out, "mode: service") {
		t.Errorf("ps:\n%s", ps.out)
	}
}

func TestServiceInstallRefusesWhileAModelIsLoaded(t *testing.T) {
	e := newEnv(t)
	e.register()
	e.run("serve").ok(t)
	e.chat("gemma4-26b-iq4xs-64k")
	e.run("service", "install").fails(t, 1, "not replacing it while it may be in use")
	if _, err := os.Stat(e.plist()); !os.IsNotExist(err) {
		t.Error("nothing should be written when install refuses")
	}
}

func TestServiceUninstallLeavesADetachedRouterAlone(t *testing.T) {
	e := newEnv(t)
	e.register()
	e.run("serve").ok(t)
	start := time.Now()
	r := e.run("service", "uninstall").ok(t)
	if !strings.Contains(r.out, "not installed") || time.Since(start) > 10*time.Second {
		t.Errorf("took %s:\n%s", time.Since(start), r.out)
	}
	if ps := e.run("ps").ok(t); !strings.Contains(ps.out, "router: up") {
		t.Errorf("the detached router must keep running:\n%s", ps.out)
	}
}

func TestServiceNeedsASubcommand(t *testing.T) {
	e := newEnv(t)
	e.run("service", "bogus").fails(t, 2, "unknown command")
}

func TestSkillInstall(t *testing.T) {
	e := newEnv(t)
	dir := filepath.Join(e.dir, "skills")
	r := e.run("skill", "install", "--dir", dir).ok(t)
	if !strings.Contains(r.out, "status: installed") {
		t.Errorf("first install:\n%s", r.out)
	}
	dest := filepath.Join(dir, "pill", "SKILL.md")
	data, err := os.ReadFile(dest)
	if err != nil || !strings.HasPrefix(string(data), "---\nname: pill\n") {
		t.Fatalf("skill file: %v\n%s", err, data)
	}
	if r := e.run("skill", "install", "--dir", dir).ok(t); !strings.Contains(r.out, "status: already installed") {
		t.Errorf("second install:\n%s", r.out)
	}
	if err := os.WriteFile(dest, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if r := e.run("skill", "install", "--dir", dir).ok(t); !strings.Contains(r.out, "status: updated") {
		t.Errorf("update:\n%s", r.out)
	}
}

func TestSkillInstallDefaultsToPiAgentDir(t *testing.T) {
	e := newEnv(t)
	e.run("skill", "install").ok(t)
	if _, err := os.Stat(filepath.Join(e.dir, "piagent", "skills", "pill", "SKILL.md")); err != nil {
		t.Error(err)
	}
}

// forgetLocalState makes this machine look fresh: models.toml stays (it is the
// stowed config) but the GGUFs and results.json are gone.
func (e *env) forgetLocalState() {
	e.t.Helper()
	e.run("stop", "--all").ok(e.t)
	for _, p := range []string{filepath.Join(e.dir, "pill", "results.json"), e.modelsDir()} {
		if err := os.RemoveAll(p); err != nil {
			e.t.Fatal(err)
		}
	}
}

func TestSyncRestoresDeclaredModelsAsUnverified(t *testing.T) {
	e := newEnv(t)
	hub := gemmaHub(t)
	e.run("pull", "gemma4-26b").ok(t)
	e.run("add", "gemma4-26b-iq4xs-64k", "--unverified").ok(t)
	e.forgetLocalState()
	if err := os.RemoveAll(filepath.Dir(e.piModels())); err != nil {
		t.Fatal(err)
	}
	hits := hub.Hits("/resolve/")

	if d := e.run("doctor"); !strings.Contains(d.out, "declared but missing locally") || !strings.Contains(d.out, "pill sync") {
		t.Errorf("doctor should point at sync:\n%s", d.out)
	}
	r := e.run("sync").ok(t)
	if !strings.Contains(r.out, "pulled[1]{model,status,size}:") || !strings.Contains(r.out, "gemma4-26b-iq4xs-64k,downloaded,") {
		t.Errorf("sync:\n%s", r.out)
	}
	if hub.Hits("/resolve/") != hits+1 {
		t.Errorf("expected exactly one download, hits %d -> %d", hits, hub.Hits("/resolve/"))
	}
	if ls := e.run("ls").ok(t); !strings.Contains(ls.out, "gemma4-26b-iq4xs-64k,unverified,") {
		t.Errorf("ls:\n%s", ls.out)
	}
	prov, _, _ := pi.ReadProvider(e.piModels())
	if !strings.Contains(string(prov), "gemma4-26b-iq4xs-64k") || !strings.Contains(string(prov), "(unverified)") {
		t.Errorf("Pi should offer it at once, labelled unverified:\n%s", prov)
	}

	again := e.run("sync").ok(t)
	if hub.Hits("/resolve/") != hits+1 || !strings.Contains(again.out, "pulled[0]:") {
		t.Errorf("a second sync must be a no-op:\n%s", again.out)
	}
}

func TestSyncBenchBenchesUnverifiedModels(t *testing.T) {
	needNode(t)
	e := newEnv(t)
	gemmaHub(t)
	solveTier2(t)
	e.run("pull", "gemma4-26b").ok(t)
	e.run("add", "gemma4-26b-iq4xs-64k", "--unverified").ok(t)
	e.forgetLocalState()

	r := e.run("sync", "--bench").ok(t)
	if !strings.Contains(r.out, "benchmarked[1]{model,result,state,dir}:") || !strings.Contains(r.out, "gemma4-26b-iq4xs-64k,passed,passed,") {
		t.Errorf("sync --bench:\n%s", r.out)
	}
	if ls := e.run("ls").ok(t); !strings.Contains(ls.out, "gemma4-26b-iq4xs-64k,passed,") {
		t.Errorf("ls:\n%s", ls.out)
	}
}

func TestSyncWithNothingDeclaredAndNoSource(t *testing.T) {
	e := newEnv(t)
	if r := e.run("sync").ok(t); !strings.Contains(r.out, "declared: 0") {
		t.Errorf("empty sync:\n%s", r.out)
	}
	// An entry with no download source cannot be fetched: reported, exit 1.
	e.register()
	e.forgetLocalState()
	toml := filepath.Join(e.dir, "pill", "config", "models.toml")
	data, err := os.ReadFile(toml)
	if err != nil {
		t.Fatal(err)
	}
	data = []byte(strings.ReplaceAll(string(data), "repo = ", "# repo = "))
	if err := os.WriteFile(toml, data, 0o644); err != nil {
		t.Fatal(err)
	}
	r := e.run("sync")
	if r.code != 1 || !strings.Contains(r.out, "skipped: no download source") {
		t.Errorf("exit %d:\n%s", r.code, r.out)
	}
}
