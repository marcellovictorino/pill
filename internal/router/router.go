// Package router starts, probes and stops llama-server in router mode.
//
// Router mode is llama-server's multi-model mode: one small always-on process
// that spawns a child per model on first request and unloads it when idle.
// Every pill command finds the router by probing its port, never by trusting
// a pid file alone, so a detached router and a launchd service look the same.
package router

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/marcellovictorino/pill/internal/config"
)

// Router talks to one llama-server router instance.
type Router struct {
	Paths    config.Paths
	Settings config.Settings
	HTTP     *http.Client
}

// New builds a Router with a short-timeout HTTP client; the router is local,
// so anything slow means it is not healthy.
func New(p config.Paths, s config.Settings) *Router {
	return &Router{Paths: p, Settings: s, HTTP: &http.Client{Timeout: 5 * time.Second}}
}

// BaseURL is the router's address, without a trailing slash.
func (r *Router) BaseURL() string {
	return "http://127.0.0.1:" + strconv.Itoa(r.Settings.Port)
}

// LlamaServer locates the llama-server executable. PILL_LLAMA_SERVER
// overrides the PATH lookup (tests point it at a fake).
func LlamaServer() (string, error) {
	if p := os.Getenv("PILL_LLAMA_SERVER"); p != "" {
		return p, nil
	}
	p, err := exec.LookPath("llama-server")
	if err != nil {
		return "", errors.New("llama-server not found on PATH")
	}
	return p, nil
}

// Args are the llama-server arguments for pill's router.
func (r *Router) Args() []string {
	return []string{
		"--models-dir", r.Settings.ModelsDir,
		"--models-preset", r.Paths.ModelsIni(),
		"--models-max", "1",
		"--sleep-idle-seconds", strconv.Itoa(r.Settings.IdleSeconds),
		"--host", "127.0.0.1",
		"--port", strconv.Itoa(r.Settings.Port),
	}
}

// Healthy reports whether something answers /health with 200 on the port.
func (r *Router) Healthy(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.BaseURL()+"/health", nil)
	if err != nil {
		return false
	}
	resp, err := r.HTTP.Do(req)
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode == http.StatusOK
}

// ModelStatus is one model as the router lists it.
type ModelStatus struct {
	ID     string
	Status string // loaded, loading, unloaded (empty when the server does not say)
}

// Models lists the models the router knows about and their load status.
func (r *Router) Models(ctx context.Context) ([]ModelStatus, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.BaseURL()+"/v1/models", nil)
	if err != nil {
		return nil, err
	}
	resp, err := r.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET /v1/models: HTTP %d", resp.StatusCode)
	}
	var body struct {
		Data []struct {
			ID     string `json:"id"`
			Status struct {
				Value string `json:"value"`
			} `json:"status"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}
	out := make([]ModelStatus, 0, len(body.Data))
	for _, m := range body.Data {
		out = append(out, ModelStatus{ID: m.ID, Status: m.Status.Value})
	}
	return out, nil
}

// Loaded returns the ids of models that hold memory: loaded, loading or unloading.
func (r *Router) Loaded(ctx context.Context) ([]string, error) {
	models, err := r.Models(ctx)
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, m := range models {
		if m.Status == "loaded" || m.Status == "loading" || m.Status == "unloading" {
			ids = append(ids, m.ID)
		}
	}
	return ids, nil
}

// Unload asks the router to unload one model, freeing its memory.
func (r *Router) Unload(ctx context.Context, id string) error {
	body, _ := json.Marshal(map[string]string{"model": id})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.BaseURL()+"/models/unload", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := r.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("unload %s: HTTP %d: %s", id, resp.StatusCode, strings.TrimSpace(string(msg)))
	}
	return nil
}

// UnloadAll unloads every loaded model and waits until the router reports
// them unloaded (unloading is asynchronous).
func (r *Router) UnloadAll(ctx context.Context) ([]string, error) {
	loaded, err := r.Loaded(ctx)
	if err != nil {
		return nil, err
	}
	for _, id := range loaded {
		if err := r.Unload(ctx, id); err != nil {
			return loaded, err
		}
	}
	deadline := time.Now().Add(30 * time.Second)
	for len(loaded) > 0 && time.Now().Before(deadline) {
		still, err := r.Loaded(ctx)
		if err != nil || len(still) == 0 {
			break
		}
		select {
		case <-ctx.Done():
			return loaded, ctx.Err()
		case <-time.After(250 * time.Millisecond):
		}
	}
	return loaded, nil
}

// --- process discovery ---

// Process describes the llama-server process answering on the port.
type Process struct {
	PID   int
	Owned bool // started with pill's models.ini
}

// Find locates the process serving the port. It prefers the pid file and
// falls back to asking the OS who listens on the port (lsof), so a router
// started by launchd or by hand is still found. Owned means its command line
// uses this pill's models.ini.
func (r *Router) Find() (Process, bool) {
	if pid, ok := r.pidFromFile(); ok {
		if cmd := commandOf(pid); strings.Contains(cmd, "llama-server") {
			return Process{PID: pid, Owned: strings.Contains(cmd, r.Paths.ModelsIni())}, true
		}
	}
	if pid, ok := listenerPID(r.Settings.Port); ok {
		cmd := commandOf(pid)
		return Process{PID: pid, Owned: strings.Contains(cmd, r.Paths.ModelsIni())}, true
	}
	return Process{}, false
}

func (r *Router) pidFromFile() (int, bool) {
	data, err := os.ReadFile(r.Paths.PidFile())
	if err != nil {
		return 0, false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 || syscall.Kill(pid, 0) != nil {
		return 0, false
	}
	return pid, true
}

func commandOf(pid int) string {
	out, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "command=").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func listenerPID(port int) (int, bool) {
	out, err := exec.Command("lsof", "-nP", "-iTCP:"+strconv.Itoa(port), "-sTCP:LISTEN", "-t").Output()
	if err != nil {
		return 0, false
	}
	first, _, _ := strings.Cut(strings.TrimSpace(string(out)), "\n")
	pid, err := strconv.Atoi(first)
	return pid, err == nil && pid > 0
}

// PortBusy reports whether anything is listening on the port (health or not).
func (r *Router) PortBusy() bool {
	_, ok := listenerPID(r.Settings.Port)
	return ok
}

// --- start and stop ---

// state is router.json: facts about the router pill started, used to notice
// when it was started from an older models.ini.
type state struct {
	PID     int       `json:"pid"`
	IniHash string    `json:"ini_hash"`
	Started time.Time `json:"started"`
}

// ReadIniHash returns the models.ini fingerprint the running router was
// started with, if pill started it.
func (r *Router) ReadIniHash() string {
	data, err := os.ReadFile(r.Paths.RouterState())
	if err != nil {
		return ""
	}
	var s state
	if json.Unmarshal(data, &s) != nil {
		return ""
	}
	if _, ok := r.pidFromFile(); !ok {
		return ""
	}
	return s.IniHash
}

// StartTimeout is how long Start waits for /health (PILL_START_TIMEOUT, seconds).
func StartTimeout() time.Duration {
	if v := os.Getenv("PILL_START_TIMEOUT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return time.Duration(n) * time.Second
		}
	}
	return 90 * time.Second
}

// Start launches llama-server detached and returns once /health answers.
//
// "Detached" means the router lives in its own session (Setsid), so closing
// the terminal or exiting pill does not kill it. Output goes to the log file.
// If the router is already healthy this does nothing and returns started=false.
func (r *Router) Start(ctx context.Context, iniHash string) (started bool, err error) {
	if r.Healthy(ctx) {
		return false, nil
	}
	if r.PortBusy() {
		return false, fmt.Errorf("port %d is in use by another process that does not answer /health", r.Settings.Port)
	}
	bin, err := LlamaServer()
	if err != nil {
		return false, err
	}
	for _, d := range []string{r.Paths.LogsDir(), r.Paths.RunDir()} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return false, err
		}
	}
	logf, err := os.OpenFile(r.Paths.ServerLog(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return false, err
	}
	defer func() { _ = logf.Close() }()

	cmd := exec.Command(bin, r.Args()...)
	cmd.Stdout, cmd.Stderr = logf, logf
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return false, fmt.Errorf("start llama-server: %w", err)
	}
	pid := cmd.Process.Pid
	_ = os.WriteFile(r.Paths.PidFile(), []byte(strconv.Itoa(pid)+"\n"), 0o644)
	st, _ := json.Marshal(state{PID: pid, IniHash: iniHash, Started: time.Now().UTC()})
	_ = os.WriteFile(r.Paths.RouterState(), st, 0o644)

	// Reap the child if it dies while we wait, so an early crash is noticed
	// instead of looking like a slow start.
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()

	timeout := time.NewTimer(StartTimeout())
	defer timeout.Stop()
	tick := time.NewTicker(200 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case werr := <-exited:
			return false, fmt.Errorf("llama-server exited during startup (%v); see %s", werr, r.Paths.ServerLog())
		case <-timeout.C:
			return false, fmt.Errorf("llama-server did not answer /health within %s; see %s", StartTimeout(), r.Paths.ServerLog())
		case <-tick.C:
			if r.Healthy(ctx) {
				return true, nil
			}
		}
	}
}

// Stop terminates the router with SIGTERM and waits for the port to free up.
// It returns false when no router was running.
func (r *Router) Stop(ctx context.Context) (stopped bool, err error) {
	proc, ok := r.Find()
	if !ok {
		_ = os.Remove(r.Paths.PidFile())
		return false, nil
	}
	if !proc.Owned {
		return false, fmt.Errorf("the process on port %d (pid %d) was not started by pill; stop it yourself", r.Settings.Port, proc.PID)
	}
	if err := syscall.Kill(proc.PID, syscall.SIGTERM); err != nil && !errors.Is(err, syscall.ESRCH) {
		return false, err
	}
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if syscall.Kill(proc.PID, 0) != nil {
			_ = os.Remove(r.Paths.PidFile())
			_ = os.Remove(r.Paths.RouterState())
			return true, nil
		}
		select {
		case <-ctx.Done():
			return true, ctx.Err()
		case <-time.After(150 * time.Millisecond):
		}
	}
	_ = syscall.Kill(proc.PID, syscall.SIGKILL)
	_ = os.Remove(r.Paths.PidFile())
	_ = os.Remove(r.Paths.RouterState())
	return true, nil
}

// ExecFunc replaces the current process; syscall.Exec in production, a
// recorder in tests.
var ExecFunc = syscall.Exec

// ExecForeground replaces pill with llama-server running in the foreground,
// for debugging (`pill serve --foreground`). It only returns on failure.
func (r *Router) ExecForeground() error {
	bin, err := LlamaServer()
	if err != nil {
		return err
	}
	return ExecFunc(bin, append([]string{"llama-server"}, r.Args()...), os.Environ())
}
