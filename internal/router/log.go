package router

import (
	"bytes"
	"errors"
	"io"
	"os"

	"github.com/marcellovictorino/pill/internal/config"
)

// Log size bounds. llama-server has no log rotation of its own, and a router
// that serves benchmarks writes timing lines for every request, so pill keeps
// server.log from growing without limit.
const (
	// LogLimit is the size above which server.log is trimmed.
	LogLimit int64 = 64 << 20
	// LogKeep is how much of the newest output is kept, in server.log.1.
	LogKeep int64 = 16 << 20
)

// TrimLog bounds the router's log file at path. Under limit it does nothing.
// Over it, the newest keep bytes (from the first whole line) replace
// path+".1", and path is emptied in place.
//
// Emptying in place rather than renaming is what makes this safe while a
// router is running: both pill and launchd open the log with O_APPEND, so the
// running process's next write lands at the new end of the file instead of
// at its old offset (which would leave a hole as large as the old file).
func TrimLog(path string, limit, keep int64) (bool, error) {
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if info.Size() <= limit {
		return false, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer func() { _ = f.Close() }()
	start := max(info.Size()-keep, 0)
	tail, err := io.ReadAll(io.NewSectionReader(f, start, info.Size()-start))
	if err != nil {
		return false, err
	}
	if start > 0 { // drop the partial first line
		if i := bytes.IndexByte(tail, '\n'); i >= 0 {
			tail = tail[i+1:]
		}
	}
	if err := config.WriteFileAtomic(path+".1", tail, 0o644); err != nil {
		return false, err
	}
	if err := os.Truncate(path, 0); err != nil {
		return false, err
	}
	return true, nil
}

// TrimLog bounds this router's server.log (see the package-level TrimLog).
func (r *Router) TrimLog() (bool, error) {
	return TrimLog(r.Paths.ServerLog(), LogLimit, LogKeep)
}
