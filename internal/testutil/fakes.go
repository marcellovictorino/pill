// Package testutil provides fake external programs for tests.
//
// Test-binary re-exec: pill's tests need stand-ins for llama-server, pi and
// launchctl. Instead of shipping scripts, a test symlinks its own test binary
// under the fake's name (for example "llama-server"). When that symlink is
// executed, TestMain calls MaybeRunFake, which sees the program name, behaves
// like the fake, and exits - the rest of the test suite never runs. This is
// the same trick the Go standard library uses to test os/exec.
package testutil

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
)

// MaybeRunFake runs a fake program when the binary was invoked under a fake's
// name. Call it first in every TestMain.
func MaybeRunFake() {
	switch filepath.Base(os.Args[0]) {
	case "llama-server":
		runFakeLlama(os.Args[1:])
		os.Exit(0)
	case "pi":
		runFakePi(os.Args[1:])
		os.Exit(0)
	case "launchctl":
		runFakeLaunchctl(os.Args[1:])
		os.Exit(0)
	}
}

// Fakes are the paths of the installed fake programs.
type Fakes struct {
	Dir         string
	LlamaServer string
	Pi          string
	Launchctl   string
	PiLog       string // fake pi appends its arguments and PI_CODING_AGENT_DIR here
	LaunchLog   string // fake launchctl appends its arguments here
}

// InstallFakes symlinks the running test binary as llama-server, pi and
// launchctl in a temp dir and points the PILL_* override variables at them.
func InstallFakes(t *testing.T) Fakes {
	t.Helper()
	dir := t.TempDir()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	f := Fakes{Dir: dir, PiLog: filepath.Join(dir, "pi.log"), LaunchLog: filepath.Join(dir, "launchctl.log")}
	for name, dst := range map[string]*string{"llama-server": &f.LlamaServer, "pi": &f.Pi, "launchctl": &f.Launchctl} {
		*dst = filepath.Join(dir, name)
		if err := os.Symlink(self, *dst); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PILL_LLAMA_SERVER", f.LlamaServer)
	t.Setenv("PILL_PI", f.Pi)
	t.Setenv("PILL_LAUNCHCTL", f.Launchctl)
	t.Setenv("PILL_FAKE_PI_LOG", f.PiLog)
	t.Setenv("PILL_FAKE_LAUNCH_LOG", f.LaunchLog)
	return f
}

// FreePort asks the OS for an unused TCP port.
func FreePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Close() }()
	return l.Addr().(*net.TCPAddr).Port
}

// --- fake llama-server (router mode) ---

func runFakeLlama(args []string) {
	var port, preset string
	for i := 0; i+1 < len(args); i++ {
		switch args[i] {
		case "--port":
			port = args[i+1]
		case "--models-preset":
			preset = args[i+1]
		}
	}
	if len(args) > 0 && args[0] == "--version" {
		fmt.Fprintln(os.Stderr, "version: 9.9.9 (build 999, commit fakefake)")
		return
	}
	ids := presetSections(preset)
	var mu sync.Mutex
	loaded := map[string]bool{}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("/v1/models", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		type status struct {
			Value string `json:"value"`
		}
		type model struct {
			ID     string `json:"id"`
			Status status `json:"status"`
		}
		var data []model
		for _, id := range ids {
			v := "unloaded"
			if loaded[id] {
				v = "loaded"
			}
			data = append(data, model{id, status{v}})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
	})
	mux.HandleFunc("/models/unload", func(w http.ResponseWriter, r *http.Request) {
		var body struct{ Model string }
		_ = json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		defer mu.Unlock()
		if !loaded[body.Model] {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"message":"model is not running"}}`))
			return
		}
		delete(loaded, body.Model)
		_, _ = w.Write([]byte(`{"success":true}`))
	})
	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		var body struct{ Model string }
		_ = json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		loaded[body.Model] = true
		mu.Unlock()
		reply := os.Getenv("PILL_FAKE_REPLY")
		if reply == "" {
			reply = "ok"
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{"message": map[string]any{"role": "assistant", "content": reply}}},
			"usage":   map[string]any{"prompt_tokens": 8000, "completion_tokens": 20},
			"timings": map[string]any{"predicted_per_second": 42.5},
		})
	})

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)
	srv := &http.Server{Addr: "127.0.0.1:" + port, Handler: mux}
	go func() { <-stop; _ = srv.Close() }()
	if _, err := strconv.Atoi(port); err != nil {
		fmt.Fprintln(os.Stderr, "fake llama-server: --port required")
		os.Exit(2)
	}
	fmt.Fprintln(os.Stderr, "fake llama-server listening on", srv.Addr)
	_ = srv.ListenAndServe()
}

// presetSections returns the [section] names of an INI file except [*].
func presetSections(path string) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer func() { _ = f.Close() }()
	var ids []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			if id := line[1 : len(line)-1]; id != "*" {
				ids = append(ids, id)
			}
		}
	}
	return ids
}

// --- fake pi ---

// The fake pi answers --version, and otherwise logs its arguments (one line)
// to $PILL_FAKE_PI_LOG. With PILL_FAKE_PI_ANSWER set it prints that text, so
// bench tests can script Pi's reply.
func runFakePi(args []string) {
	if len(args) > 0 && args[0] == "--version" {
		fmt.Println("9.9.9")
		return
	}
	if log := os.Getenv("PILL_FAKE_PI_LOG"); log != "" {
		f, err := os.OpenFile(log, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err == nil {
			fmt.Fprintf(f, "dir=%s args=%s\n", os.Getenv("PI_CODING_AGENT_DIR"), strings.Join(args, " "))
			_ = f.Close()
		}
	}
	if script := os.Getenv("PILL_FAKE_PI_SCRIPT"); script != "" {
		runPiScript(script)
		return
	}
	fmt.Println(os.Getenv("PILL_FAKE_PI_ANSWER"))
}

// runPiScript runs a shell command in the current directory; bench tests use
// it to make the fake Pi "write code" into the workspace.
func runPiScript(script string) {
	sh, _ := os.StartProcess("/bin/sh", []string{"sh", "-c", script}, &os.ProcAttr{
		Files: []*os.File{os.Stdin, os.Stdout, os.Stderr}, Env: os.Environ(),
	})
	if sh != nil {
		_, _ = sh.Wait()
	}
}

// --- fake launchctl ---

func runFakeLaunchctl(args []string) {
	if log := os.Getenv("PILL_FAKE_LAUNCH_LOG"); log != "" {
		f, err := os.OpenFile(log, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err == nil {
			fmt.Fprintln(f, strings.Join(args, " "))
			_ = f.Close()
		}
	}
	// "print" fails unless a marker file says the job is loaded.
	if len(args) > 0 && args[0] == "print" {
		if m := os.Getenv("PILL_FAKE_LAUNCH_LOADED"); m != "" {
			if _, err := os.Stat(m); err == nil {
				return
			}
		}
		os.Exit(113)
	}
}
