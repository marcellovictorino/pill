package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/marcellovictorino/pill/internal/pi"
)

// These tests cover the 30 October 2026 benchmark incident: a smoke test loaded
// a second model into a router that was serving a benchmark, and a Pi agent
// outlived the benchmark that was killed.

const otherFile = "other-Q4_K_M.gguf"

// piExecRecorder replaces the exec of Pi and reports whether it was reached.
func piExecRecorder(t *testing.T) *bool {
	t.Helper()
	reached := false
	orig := pi.ExecFunc
	t.Cleanup(func() { pi.ExecFunc = orig })
	pi.ExecFunc = func(string, []string, []string) error { reached = true; return nil }
	return &reached
}

func TestPiRefusesWhileAnotherModelIsLoaded(t *testing.T) {
	e := newEnv(t)
	e.register()
	e.gguf(otherFile)
	e.run("add", otherFile, "--unverified", "--ctx", "8192").ok(t)
	e.run("serve").ok(t)
	e.chat("other-q4-k-m-8k")
	reached := piExecRecorder(t)

	e.run().fails(t, 1, "other-q4-k-m-8k")
	if *reached {
		t.Error("pi was started although another model holds the router")
	}
	if loaded, _ := e.router().Loaded(t.Context()); len(loaded) != 1 || loaded[0] != "other-q4-k-m-8k" {
		t.Errorf("the model that was loaded must stay loaded, got %v", loaded)
	}

	// The default model itself being loaded is the ordinary case: allowed.
	e.run("stop").ok(t)
	e.chat("gemma4-26b-iq4xs-64k")
	e.run()
	if !*reached {
		t.Error("pi must start when the loaded model is the default")
	}
}

// slowBench starts `pill bench run` as its own process with a fake Pi that
// starts a child and then waits, and returns once Tier 2 is running. The
// child's pid is written to childPid.
func (e *env) slowBench(t *testing.T) (pill *exec.Cmd, childPid string) {
	t.Helper()
	needNode(t)
	childPid = filepath.Join(e.dir, "child.pid")
	t.Setenv("PILL_FAKE_PI_TIER2_SCRIPT", "sleep 300 & echo $! > '"+childPid+"'; wait")
	e.gguf(gemmaFile)
	e.run("add", "gemma4-26b-iq4xs-64k", "--unverified").ok(t)
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	pill = exec.Command(self, "bench", "run", "gemma4-26b-iq4xs-64k")
	pill.Env = append(os.Environ(), "PILL_TEST_AS_PILL=1")
	if err := pill.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = pill.Process.Kill()
		_ = pill.Wait()
		if pid := readPid(childPid); pid > 0 {
			_ = syscall.Kill(pid, syscall.SIGKILL)
		}
	})
	waitFor(t, func() bool { return readPid(childPid) > 0 })
	return pill, childPid
}

func readPid(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	pid, _ := strconv.Atoi(strings.TrimSpace(string(data)))
	return pid
}

func alive(pid int) bool { return syscall.Kill(pid, 0) == nil }

func TestSecondBenchmarkAndPiAreRefusedWhileOneRuns(t *testing.T) {
	e := newEnv(t)
	pill, childPid := e.slowBench(t)
	e.run("default", "gemma4-26b-iq4xs-64k").ok(t)
	reached := piExecRecorder(t)

	e.run("bench", "run", "gemma4-26b-iq4xs-64k").fails(t, 1, "another pill benchmark is running")
	e.run().fails(t, 1, "another pill benchmark is running")
	if *reached {
		t.Error("pi was started during a benchmark")
	}
	if !alive(readPid(childPid)) || pill.ProcessState != nil {
		t.Error("the refused commands must not disturb the running benchmark")
	}
}

func TestHangupEndsPiAndItsChildren(t *testing.T) {
	e := newEnv(t)
	pill, childPid := e.slowBench(t)
	child := readPid(childPid)

	if err := pill.Process.Signal(syscall.SIGHUP); err != nil {
		t.Fatal(err)
	}
	_ = pill.Wait()
	waitFor(t, func() bool { return !alive(child) })
}

func TestBenchmarkStartEndsAgentsLeftByAKilledBenchmark(t *testing.T) {
	e := newEnv(t)
	pill, childPid := e.slowBench(t)
	child := readPid(childPid)

	// SIGKILL cannot be handled, so the agent is orphaned until the next run.
	_ = pill.Process.Signal(syscall.SIGKILL)
	_ = pill.Wait()
	if !alive(child) {
		t.Fatal("setup: the orphan should survive a hard kill")
	}

	solveTier2(t)
	e.run("bench", "run", "gemma4-26b-iq4xs-64k").ok(t)
	deadline := time.Now().Add(5 * time.Second)
	for alive(child) && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	if alive(child) {
		t.Error("the next benchmark left the previous benchmark's agent running")
	}
}
