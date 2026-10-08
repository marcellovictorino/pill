package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/marcellovictorino/pill/internal/config"
	"github.com/marcellovictorino/pill/internal/testutil"
)

func TestMain(m *testing.M) {
	testutil.MaybeRunFake() // behaves as the fake llama-server when invoked under that name
	os.Exit(m.Run())
}

// serverFor starts an httptest server posing as the router and returns a
// Router pointed at it.
func serverFor(t *testing.T, h http.Handler) *Router {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	port, _ := strconv.Atoi(srv.URL[strings.LastIndex(srv.URL, ":")+1:])
	root := t.TempDir()
	return New(config.Paths{ConfigDir: filepath.Join(root, "c"), Root: root},
		config.Settings{Port: port, IdleSeconds: 900, ModelsDir: filepath.Join(root, "models")})
}

func TestHealthyAndModels(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`{"status":"ok"}`)) })
	mux.HandleFunc("/v1/models", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"id":"a","status":{"value":"loaded"}},{"id":"b","status":{"value":"unloaded"}},{"id":"c","status":{"value":"loading"}}]}`))
	})
	r := serverFor(t, mux)
	ctx := context.Background()
	if !r.Healthy(ctx) {
		t.Fatal("expected healthy")
	}
	models, err := r.Models(ctx)
	if err != nil || len(models) != 3 || models[1].Status != "unloaded" {
		t.Fatalf("models = %+v, %v", models, err)
	}
	loaded, _ := r.Loaded(ctx)
	if strings.Join(loaded, ",") != "a,c" {
		t.Errorf("loaded = %v", loaded)
	}
}

func TestNotHealthyWhenNothingListens(t *testing.T) {
	r := New(config.Paths{Root: t.TempDir()}, config.Settings{Port: testutil.FreePort(t)})
	if r.Healthy(context.Background()) {
		t.Error("nothing is listening, Healthy must be false")
	}
}

func TestUnloadAllWaitsUntilUnloaded(t *testing.T) {
	var mu sync.Mutex
	status := "loaded"
	polls := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/models", func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if status == "unloading" {
			polls++
			if polls >= 2 {
				status = "unloaded"
			}
		}
		_, _ = w.Write([]byte(`{"data":[{"id":"m","status":{"value":"` + status + `"}}]}`))
	})
	mux.HandleFunc("/models/unload", func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		status = "unloading"
		mu.Unlock()
		_, _ = w.Write([]byte(`{}`))
	})
	r := serverFor(t, mux)
	unloaded, err := r.UnloadAll(context.Background())
	if err != nil || len(unloaded) != 1 || unloaded[0] != "m" {
		t.Fatalf("unloaded=%v err=%v", unloaded, err)
	}
	mu.Lock()
	defer mu.Unlock()
	if status != "unloaded" {
		t.Errorf("status = %s", status)
	}
}

func TestUnloadReportsServerError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/models/unload", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`model is not running`))
	})
	r := serverFor(t, mux)
	err := r.Unload(context.Background(), "x")
	if err == nil || !strings.Contains(err.Error(), "not running") {
		t.Errorf("err = %v", err)
	}
}

func TestArgs(t *testing.T) {
	r := New(config.Paths{Root: "/root"}, config.Settings{Port: 11435, IdleSeconds: 900, ModelsDir: "/root/models"})
	got := strings.Join(r.Args(), " ")
	want := "--models-dir /root/models --models-preset /root/models.ini --models-max 1 --sleep-idle-seconds 900 --host 127.0.0.1 --port 11435"
	if got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

// newRealRouter wires a Router to the fake llama-server binary.
func newRealRouter(t *testing.T) *Router {
	t.Helper()
	testutil.InstallFakes(t)
	root := t.TempDir()
	p := config.Paths{ConfigDir: filepath.Join(root, "config"), Root: root}
	if err := os.WriteFile(p.ModelsIni(), []byte("[*]\njinja = true\n\n[m1]\nmodel = /x.gguf\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return New(p, config.Settings{Port: testutil.FreePort(t), IdleSeconds: 900, ModelsDir: filepath.Join(root, "models")})
}

func TestStartIsIdempotentAndStopWorks(t *testing.T) {
	r := newRealRouter(t)
	ctx := context.Background()
	t.Cleanup(func() { _, _ = r.Stop(ctx) })

	started, err := r.Start(ctx, "hash1")
	if err != nil || !started {
		t.Fatalf("first start: started=%v err=%v", started, err)
	}
	if got := r.ReadIniHash(); got != "hash1" {
		t.Errorf("ini hash = %q", got)
	}
	proc, ok := r.Find()
	if !ok || !proc.Owned {
		t.Fatalf("Find = %+v %v", proc, ok)
	}
	started, err = r.Start(ctx, "hash1")
	if err != nil || started {
		t.Errorf("second start must be a no-op: started=%v err=%v", started, err)
	}
	models, _ := r.Models(ctx)
	if len(models) != 1 || models[0].ID != "m1" {
		t.Errorf("models = %+v", models)
	}

	stopped, err := r.Stop(ctx)
	if err != nil || !stopped {
		t.Fatalf("stop: stopped=%v err=%v", stopped, err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for r.Healthy(ctx) && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	if r.Healthy(ctx) {
		t.Error("router still answering after Stop")
	}
	if stopped, _ := r.Stop(ctx); stopped {
		t.Error("stopping a stopped router must report false")
	}
}

func TestStartFailsWhenBinaryExitsEarly(t *testing.T) {
	r := newRealRouter(t)
	t.Setenv("PILL_LLAMA_SERVER", "/usr/bin/false")
	t.Setenv("PILL_START_TIMEOUT", "5")
	_, err := r.Start(context.Background(), "h")
	if err == nil || !strings.Contains(err.Error(), "exited during startup") {
		t.Errorf("err = %v", err)
	}
}

func TestUnloadAllFailsWhenTheStatusPollFails(t *testing.T) {
	var mu sync.Mutex
	unloaded := false
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/models", func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if unloaded { // the first poll saw a loaded model; the confirming poll breaks
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(`{"data":[{"id":"m","status":{"value":"loaded"}}]}`))
	})
	mux.HandleFunc("/models/unload", func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		unloaded = true
		mu.Unlock()
		_, _ = w.Write([]byte(`{}`))
	})
	r := serverFor(t, mux)
	if _, err := r.UnloadAll(context.Background()); err == nil || !strings.Contains(err.Error(), "cannot confirm") {
		t.Errorf("a failed confirmation poll must be an error, got %v", err)
	}
}

func TestUnloadAllTimesOutWhileAModelStaysLoaded(t *testing.T) {
	old := unloadTimeout
	unloadTimeout = 600 * time.Millisecond
	t.Cleanup(func() { unloadTimeout = old })
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/models", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"id":"stuck","status":{"value":"loaded"}}]}`))
	})
	mux.HandleFunc("/models/unload", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`{}`)) })
	r := serverFor(t, mux)
	if _, err := r.UnloadAll(context.Background()); err == nil || !strings.Contains(err.Error(), "stuck is still loaded") {
		t.Errorf("a model that never leaves must time out with an error, got %v", err)
	}
}

func TestFindIgnoresAPidFileForAnotherPort(t *testing.T) {
	a := newRealRouter(t)
	ctx := context.Background()
	t.Cleanup(func() { _, _ = a.Stop(ctx) })
	if _, err := a.Start(ctx, "h"); err != nil {
		t.Fatal(err)
	}
	// A second router config sharing the same run directory (and so the same
	// pid file) but another port, as with PILL_PORT: the pid file now names a
	// live llama-server that does not serve this port.
	b := New(a.Paths, config.Settings{Port: testutil.FreePort(t), IdleSeconds: 900, ModelsDir: a.Settings.ModelsDir})
	if proc, ok := b.Find(); ok {
		t.Errorf("Find for an unused port returned %+v from the shared pid file", proc)
	}
	if stopped, err := b.Stop(ctx); stopped || err != nil {
		t.Errorf("stopping the other port must not touch the first router: stopped=%v err=%v", stopped, err)
	}
	if !a.Healthy(ctx) {
		t.Error("the first router was stopped through the shared pid file")
	}
}

func TestRecordedFingerprintBelongsToTheRunningProcess(t *testing.T) {
	r := newRealRouter(t)
	ctx := context.Background()
	t.Cleanup(func() { _, _ = r.Stop(ctx) })
	if _, err := r.Start(ctx, "old"); err != nil {
		t.Fatal(err)
	}
	// Rewrite the record as if it described a different (dead) process: a
	// router restarted by launchd on its own must not inherit the stale hash.
	st := `{"pid":999999,"ini_hash":"old"}`
	if err := os.WriteFile(r.Paths.RouterState(), []byte(st), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := r.ReadIniHash(); got != "" {
		t.Errorf("hash of another process was accepted: %q", got)
	}
	r.RecordStart("new")
	if got := r.ReadIniHash(); got != "new" {
		t.Errorf("after RecordStart hash = %q", got)
	}
}
