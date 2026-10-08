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
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
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
	t.Setenv("PILL_FAKE_LAUNCH_STATE", filepath.Join(dir, "launch.state"))
	t.Setenv("PILL_LAUNCH_AGENTS_DIR", filepath.Join(dir, "LaunchAgents"))
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
		build := os.Getenv("PILL_FAKE_LLAMA_BUILD") // lets a test "upgrade" llama.cpp
		if build == "" {
			build = "999"
		}
		fmt.Fprintf(os.Stderr, "version: 9.9.9 (build %s, commit fakefake)\n", build)
		return
	}
	ids := presetSections(preset)
	var mu sync.Mutex
	loaded := map[string]bool{}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		// While the file named by PILL_FAKE_UNHEALTHY exists the router answers
		// 503, like a llama-server that is still loading or has hung.
		if f := os.Getenv("PILL_FAKE_UNHEALTHY"); f != "" {
			if _, err := os.Stat(f); err == nil {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
		}
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

// The fake pi answers --version and otherwise behaves like `pi -p --mode json`:
//   - it logs its arguments and PI_CODING_AGENT_DIR to $PILL_FAKE_PI_LOG;
//   - for the Tier 1 prompt it "reads" facts.txt (one tool call) and answers
//     with its content; PILL_FAKE_PI_TIER1=wrong answers incorrectly and
//     PILL_FAKE_PI_TIER1=notool skips the tool call;
//   - for the Tier 2 prompt it runs the shell command in PILL_FAKE_PI_TIER2_SCRIPT
//     (which can write files into the workspace, standing in for the model)
//     after sleeping PILL_FAKE_PI_TIER2_SLEEP seconds (to test the time cap).
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
	prompt := ""
	if len(args) > 0 {
		prompt = args[len(args)-1]
	}

	answer, tools := os.Getenv("PILL_FAKE_PI_ANSWER"), 0
	switch {
	case strings.Contains(prompt, "facts.txt"):
		data, _ := os.ReadFile("facts.txt")
		answer, tools = strings.TrimSpace(string(data)), 1
		switch os.Getenv("PILL_FAKE_PI_TIER1") {
		case "wrong":
			answer = "PILL-00000000"
		case "notool":
			tools = 0
		}
	case strings.Contains(prompt, "tic-tac-toe"):
		if n, _ := strconv.Atoi(os.Getenv("PILL_FAKE_PI_TIER2_SLEEP")); n > 0 {
			time.Sleep(time.Duration(n) * time.Second)
		}
		if script := os.Getenv("PILL_FAKE_PI_TIER2_SCRIPT"); script != "" {
			runShell(script)
		}
		answer, tools = "done", 3
	}

	for i := 0; i < tools; i++ {
		fmt.Println(`{"type":"tool_execution_start","toolName":"read"}`)
	}
	msg, _ := json.Marshal(map[string]any{"type": "message_end", "message": map[string]any{
		"role": "assistant", "content": []map[string]string{{"type": "text", "text": answer}},
	}})
	fmt.Println(string(msg))
	fmt.Println(`{"type":"agent_settled"}`)
}

// runShell runs a shell command in the current directory.
func runShell(script string) {
	p, err := os.StartProcess("/bin/sh", []string{"sh", "-c", script}, &os.ProcAttr{
		Files: []*os.File{os.Stdin, os.Stdout, os.Stderr}, Env: os.Environ(),
	})
	if err == nil {
		_, _ = p.Wait()
	}
}

// --- fake launchctl ---

// The fake models just enough of launchd: bootstrap starts the program named
// in the plist (the fake llama-server) and remembers its pid; bootout stops
// it; kickstart -k restarts it; print succeeds while the job is loaded.
func runFakeLaunchctl(args []string) {
	if log := os.Getenv("PILL_FAKE_LAUNCH_LOG"); log != "" {
		f, err := os.OpenFile(log, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err == nil {
			fmt.Fprintln(f, strings.Join(args, " "))
			_ = f.Close()
		}
	}
	state := os.Getenv("PILL_FAKE_LAUNCH_STATE") // file holding "<pid> <plist>" while loaded
	if state == "" || len(args) == 0 {
		return
	}
	loaded := func() (pid int, plist string, ok bool) {
		data, err := os.ReadFile(state)
		if err != nil {
			return 0, "", false
		}
		f := strings.Fields(string(data))
		if len(f) != 2 {
			return 0, "", false
		}
		pid, _ = strconv.Atoi(f[0])
		return pid, f[1], pid > 0 && syscall.Kill(pid, 0) == nil
	}
	start := func(plist string) {
		data, err := os.ReadFile(plist)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Bootstrap failed: 5: Input/output error")
			os.Exit(5)
		}
		var argv []string
		text := string(data)
		if i := strings.Index(text, "<key>ProgramArguments</key>"); i >= 0 {
			text = text[i:]
			text = text[:strings.Index(text, "</array>")]
			for _, part := range strings.Split(text, "<string>")[1:] {
				argv = append(argv, strings.Split(part, "</string>")[0])
			}
		}
		if len(argv) == 0 {
			os.Exit(5)
		}
		cmd := exec.Command(argv[0], argv[1:]...)
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		if err := cmd.Start(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(5)
		}
		_ = os.WriteFile(state, []byte(fmt.Sprintf("%d %s", cmd.Process.Pid, plist)), 0o644)
	}
	stop := func() {
		if pid, _, ok := loaded(); ok {
			_ = syscall.Kill(pid, syscall.SIGTERM)
		}
	}
	switch args[0] {
	case "print":
		if _, _, ok := loaded(); !ok {
			os.Exit(113)
		}
	case "bootstrap": // bootstrap gui/<uid> <plist>
		if _, _, ok := loaded(); ok {
			fmt.Fprintln(os.Stderr, "Bootstrap failed: 5: Input/output error")
			os.Exit(5)
		}
		start(args[len(args)-1])
	case "bootout":
		if _, _, ok := loaded(); !ok {
			fmt.Fprintln(os.Stderr, "Boot-out failed: 3: No such process")
			os.Exit(3)
		}
		stop()
		_ = os.Remove(state)
	case "kickstart":
		if _, plist, ok := loaded(); ok {
			for _, a := range args {
				if a == "-k" {
					stop()
					time.Sleep(300 * time.Millisecond)
					start(plist)
				}
			}
		}
	}
}
