package bench

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

// ErrBusy means another benchmark holds the lock.
var ErrBusy = errors.New("another pill benchmark is running")

// Lock is the one-benchmark-at-a-time lock. It is a flock on a file, so the
// kernel drops it when the holder dies, however it dies. The file also records
// the process group of the Pi agent currently running, so the next holder can
// end an agent that outlived a benchmark killed with SIGKILL.
type Lock struct {
	f *os.File
}

type lockRecord struct {
	Pgid int `json:"pgid,omitempty"`
}

// Held reports whether a benchmark holds the lock at path.
func Held(path string) bool {
	f, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()
	return syscall.Flock(int(f.Fd()), syscall.LOCK_SH|syscall.LOCK_NB) != nil
}

// Acquire takes the lock or returns ErrBusy. Having taken it, it ends any
// agent process group left behind by a previous holder that died.
func Acquire(path string) (*Lock, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, ErrBusy
		}
		return nil, err
	}
	l := &Lock{f: f}
	var rec lockRecord
	if data, err := os.ReadFile(path); err == nil && json.Unmarshal(data, &rec) == nil && rec.Pgid > 0 {
		_ = syscall.Kill(-rec.Pgid, syscall.SIGKILL)
	}
	l.record(0)
	return l, nil
}

// record stores the agent's process group (0 when none is running).
func (l *Lock) record(pgid int) {
	if l == nil {
		return
	}
	data, _ := json.Marshal(lockRecord{Pgid: pgid})
	_ = l.f.Truncate(0)
	_, _ = l.f.WriteAt(append(data, '\n'), 0)
}

// Release drops the lock.
func (l *Lock) Release() {
	if l == nil {
		return
	}
	l.record(0)
	_ = l.f.Close()
}
