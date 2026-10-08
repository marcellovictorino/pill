package config

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// LockState takes an exclusive, cross-process lock on pill's model state
// (models.toml and results.json) and returns the function that releases it.
//
// Several pill commands read the state, change it and write it back (add, rm,
// pull, sync, and a benchmark's final verdict). Without a lock a command that
// finishes later writes back what it read earlier and silently undoes the
// others' changes. flock(2) is the operating system's advisory file lock: it
// is released automatically if the process dies, so a crash never leaves the
// state locked.
func LockState(ctx context.Context, p Paths) (unlock func(), err error) {
	return lockFile(ctx, p, p.StateLock(), "changing the model list")
}

// LockFileName locks one downloaded file's name, so two pulls of the same
// destination (from different repositories) cannot interleave their
// check-download-record steps.
func LockFileName(ctx context.Context, p Paths, name string) (unlock func(), err error) {
	return lockFile(ctx, p, filepath.Join(p.RunDir(), "pull-"+name+".lock"), "downloading "+name)
}

func lockFile(ctx context.Context, p Paths, path, doing string) (unlock func(), err error) {
	if err := os.MkdirAll(p.RunDir(), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	deadline := time.Now().Add(2 * time.Minute)
	for {
		err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			return func() {
				_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
				_ = f.Close()
			}, nil
		}
		if err != syscall.EWOULDBLOCK {
			_ = f.Close()
			return nil, err
		}
		if time.Now().After(deadline) {
			_ = f.Close()
			return nil, fmt.Errorf("another pill command is %s (lock %s); try again in a moment", doing, path)
		}
		select {
		case <-ctx.Done():
			_ = f.Close()
			return nil, ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}
