package bench

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// piRun is the outcome of one non-interactive Pi invocation.
type piRun struct {
	Answer     string
	ToolCalls  int
	Repeats    int // longest run of identical consecutive tool calls (a stuck agent)
	Duration   time.Duration
	TimedOut   bool
	Err        error
	Transcript string // path of the saved JSON event stream
}

// runPi runs `pi -p` in workspace against the throwaway agent dir and parses
// its JSON event stream. The Pi process gets its own process group, so on
// timeout the whole tree (Pi plus any command it started) is killed.
func (r *Runner) runPi(ctx context.Context, timeout time.Duration, workspace, agentDir, model, prompt, outDir, label string) piRun {
	out := piRun{Transcript: filepath.Join(outDir, label+"-pi.jsonl")}
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		out.Err = err
		return out
	}
	tf, err := os.Create(out.Transcript)
	if err != nil {
		out.Err = err
		return out
	}
	defer func() { _ = tf.Close() }()
	ef, err := os.Create(filepath.Join(outDir, label+"-pi.stderr.log"))
	if err != nil {
		out.Err = err
		return out
	}
	defer func() { _ = ef.Close() }()

	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	args := []string{
		"-p", "--mode", "json", "--no-session", "--offline",
		"--no-extensions", "--no-skills", "--no-context-files",
		"--model", "pill/" + model, "--thinking", "off", prompt,
	}
	cmd := exec.CommandContext(runCtx, r.PiBin, args...)
	cmd.Dir = workspace
	cmd.Env = append(os.Environ(), "PI_CODING_AGENT_DIR="+agentDir, "PI_SKIP_VERSION_CHECK=1")
	// Stdin stays nil, which Go wires to /dev/null: Pi waits on an open,
	// non-terminal stdin, so inheriting ours would hang the benchmark.
	cmd.Stdout, cmd.Stderr = tf, ef
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = 5 * time.Second

	start := time.Now()
	err = cmd.Run()
	out.Duration = time.Since(start)
	if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		out.TimedOut = true
	} else if err != nil && ctx.Err() == nil {
		out.Err = fmt.Errorf("pi exited: %w", err)
	}
	out.Answer, out.ToolCalls, out.Repeats = parsePiOutput(out.Transcript)
	return out
}

// parsePiOutput extracts the final assistant text and the number of tool
// calls from Pi's --mode json event stream, plus the longest run of identical
// consecutive tool calls (small models sometimes repeat one command forever).
// If the file is not an event
// stream (a plain-text fake, or a Pi without JSON mode), the whole text is the
// answer and no tool calls are counted.
func parsePiOutput(path string) (answer string, toolCalls, repeats int) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", 0, 0
	}
	dec := json.NewDecoder(bufio.NewReader(bytes.NewReader(data)))
	events, run := 0, 0
	lastCall := ""
	for {
		var e struct {
			Type     string          `json:"type"`
			ToolName string          `json:"toolName"`
			Args     json.RawMessage `json:"args"`
			Message  *struct {
				Role    string          `json:"role"`
				Content json.RawMessage `json:"content"`
			} `json:"message"`
		}
		if err := dec.Decode(&e); err != nil {
			if err == io.EOF {
				break
			}
			if events == 0 {
				return strings.TrimSpace(string(data)), 0, 0
			}
			break
		}
		events++
		switch {
		case e.Type == "tool_execution_start":
			toolCalls++
			call := e.ToolName + string(e.Args)
			if call == lastCall {
				run++
			} else {
				lastCall, run = call, 1
			}
			repeats = max(repeats, run)
		case e.Type == "message_end" && e.Message != nil && e.Message.Role == "assistant":
			if text := contentText(e.Message.Content); text != "" {
				answer = text
			}
		}
	}
	return strings.TrimSpace(answer), toolCalls, repeats
}

// contentText flattens a message content field: a plain string, or an array
// of parts of which only {"type":"text"} parts count.
func contentText(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &parts) != nil {
		return ""
	}
	var b strings.Builder
	for _, p := range parts {
		if p.Type == "text" {
			b.WriteString(p.Text)
		}
	}
	return b.String()
}
