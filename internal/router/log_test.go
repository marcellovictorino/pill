package router

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTrimLogLeavesASmallLogAlone(t *testing.T) {
	path := filepath.Join(t.TempDir(), "server.log")
	if err := os.WriteFile(path, []byte("line 1\nline 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	trimmed, err := TrimLog(path, 100, 10)
	if err != nil || trimmed {
		t.Fatalf("trimmed=%v err=%v, want nothing done under the limit", trimmed, err)
	}
	if got, _ := os.ReadFile(path); string(got) != "line 1\nline 2\n" {
		t.Errorf("log changed: %q", got)
	}
	if _, err := os.Stat(path + ".1"); !os.IsNotExist(err) {
		t.Errorf("no backup should be written under the limit (stat err %v)", err)
	}
}

func TestTrimLogMissingFileIsNotAnError(t *testing.T) {
	if trimmed, err := TrimLog(filepath.Join(t.TempDir(), "server.log"), 1, 1); err != nil || trimmed {
		t.Fatalf("trimmed=%v err=%v", trimmed, err)
	}
}

// The point of trimming is that a router left running cannot fill the disk,
// so it has to work while llama-server still holds the file open for append,
// keep the newest whole lines for reading, and not leave a hole the size of
// the old file when the router writes again.
func TestTrimLogKeepsTheNewestLinesAndTheRunningRouterKeepsLogging(t *testing.T) {
	path := filepath.Join(t.TempDir(), "server.log")
	// The router's own handle: pill and launchd both open the log like this.
	router, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = router.Close() }()
	var lines []string
	for i := range 200 {
		lines = append(lines, "request "+strings.Repeat("x", i%7)+" done")
	}
	if _, err := router.WriteString(strings.Join(lines, "\n") + "\n"); err != nil {
		t.Fatal(err)
	}

	trimmed, err := TrimLog(path, 1000, 305)
	if err != nil || !trimmed {
		t.Fatalf("trimmed=%v err=%v, want the oversized log trimmed", trimmed, err)
	}

	kept, err := os.ReadFile(path + ".1")
	if err != nil {
		t.Fatal(err)
	}
	if len(kept) == 0 || len(kept) > 305 {
		t.Fatalf("kept %d bytes, want the newest output up to 305", len(kept))
	}
	keptLines := strings.Split(strings.TrimSuffix(string(kept), "\n"), "\n")
	if want := lines[len(lines)-len(keptLines):]; strings.Join(keptLines, "\n") != strings.Join(want, "\n") {
		t.Errorf("kept lines are not the newest whole lines:\n%s", kept)
	}

	if _, err := router.WriteString("after trim\n"); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); string(got) != "after trim\n" {
		t.Errorf("server.log after the router writes again = %q (%d bytes), want only the new line", got, len(got))
	}
}
