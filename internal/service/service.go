// Package service manages pill's optional launchd LaunchAgent.
//
// launchd is macOS's process supervisor. A LaunchAgent is a per-user job
// described by a property list (plist) in ~/Library/LaunchAgents. With
// RunAtLoad and KeepAlive the router starts at login and is restarted if it
// ever dies. The agent runs llama-server directly, so launchd supervises the
// real process and pill's other commands find it by probing the port exactly
// as they would a detached router.
package service

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/marcellovictorino/pill/internal/config"
)

// Label identifies the job to launchd.
const Label = "com.marcellovictorino.pill"

// Service is the LaunchAgent for one pill installation.
type Service struct {
	Paths config.Paths
	UID   int
}

// New returns the service for these paths and the current user.
func New(p config.Paths) *Service { return &Service{Paths: p, UID: os.Getuid()} }

// Launchctl is the launchctl executable; PILL_LAUNCHCTL replaces it in tests.
func Launchctl() string {
	if p := os.Getenv("PILL_LAUNCHCTL"); p != "" {
		return p
	}
	return "launchctl"
}

// AgentsDir is where user LaunchAgents live (PILL_LAUNCH_AGENTS_DIR in tests).
func AgentsDir() (string, error) {
	if d := os.Getenv("PILL_LAUNCH_AGENTS_DIR"); d != "" {
		return d, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents"), nil
}

// PlistPath is the agent's property list.
func (s *Service) PlistPath() (string, error) {
	dir, err := AgentsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, Label+".plist"), nil
}

// domain is launchd's name for this user's GUI session.
func (s *Service) domain() string { return fmt.Sprintf("gui/%d", s.UID) }

// target addresses the job inside the domain.
func (s *Service) target() string { return s.domain() + "/" + Label }

// Plist renders the property list. bin and args are the llama-server
// command; the log file receives its output.
func Plist(bin string, args []string, workDir, logPath, path string) []byte {
	var b bytes.Buffer
	esc := func(s string) string {
		var e bytes.Buffer
		_ = xml.EscapeText(&e, []byte(s))
		return e.String()
	}
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
`)
	fmt.Fprintf(&b, "\t<key>Label</key>\n\t<string>%s</string>\n", esc(Label))
	b.WriteString("\t<key>ProgramArguments</key>\n\t<array>\n")
	for _, a := range append([]string{bin}, args...) {
		fmt.Fprintf(&b, "\t\t<string>%s</string>\n", esc(a))
	}
	b.WriteString("\t</array>\n")
	fmt.Fprintf(&b, "\t<key>WorkingDirectory</key>\n\t<string>%s</string>\n", esc(workDir))
	b.WriteString("\t<key>RunAtLoad</key>\n\t<true/>\n\t<key>KeepAlive</key>\n\t<true/>\n")
	fmt.Fprintf(&b, "\t<key>StandardOutPath</key>\n\t<string>%s</string>\n", esc(logPath))
	fmt.Fprintf(&b, "\t<key>StandardErrorPath</key>\n\t<string>%s</string>\n", esc(logPath))
	// launchd gives agents a minimal PATH; llama-server may spawn helpers.
	fmt.Fprintf(&b, "\t<key>EnvironmentVariables</key>\n\t<dict>\n\t\t<key>PATH</key>\n\t\t<string>%s</string>\n\t</dict>\n", esc(path))
	b.WriteString("\t<key>ProcessType</key>\n\t<string>Interactive</string>\n</dict>\n</plist>\n")
	return b.Bytes()
}

// Installed reports whether the plist exists.
func (s *Service) Installed() bool {
	p, err := s.PlistPath()
	if err != nil {
		return false
	}
	_, err = os.Stat(p)
	return err == nil
}

// Loaded reports whether launchd currently knows the job (it may be installed
// but booted out, for example after `pill stop --all`).
func (s *Service) Loaded(ctx context.Context) bool {
	return exec.CommandContext(ctx, Launchctl(), "print", s.target()).Run() == nil
}

// run executes launchctl and folds its output into the error.
func (s *Service) run(ctx context.Context, args ...string) error {
	out, err := exec.CommandContext(ctx, Launchctl(), args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("launchctl %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

// Install writes the plist and loads the job, so the router starts now and
// at every login. Installing again replaces the previous definition.
func (s *Service) Install(ctx context.Context, bin string, args []string, path string) (string, error) {
	plist, err := s.PlistPath()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(plist), 0o755); err != nil {
		return "", err
	}
	for _, d := range []string{s.Paths.LogsDir(), s.Paths.RunDir()} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return "", err
		}
	}
	if s.Loaded(ctx) {
		if err := s.run(ctx, "bootout", s.target()); err != nil {
			return "", err
		}
	}
	data := Plist(bin, args, s.Paths.Root, s.Paths.ServerLog(), path)
	if err := config.WriteFileAtomic(plist, data, 0o644); err != nil {
		return "", err
	}
	if err := s.run(ctx, "bootstrap", s.domain(), plist); err != nil {
		return plist, err
	}
	return plist, nil
}

// Uninstall unloads the job and deletes the plist. Not installed is a no-op
// (returns false).
func (s *Service) Uninstall(ctx context.Context) (bool, error) {
	plist, err := s.PlistPath()
	if err != nil {
		return false, err
	}
	was := s.Installed()
	if s.Loaded(ctx) {
		if err := s.run(ctx, "bootout", s.target()); err != nil {
			return was, err
		}
	}
	if err := os.Remove(plist); err != nil && !errors.Is(err, os.ErrNotExist) {
		return was, err
	}
	return was, nil
}

// Start loads the job (when booted out) or restarts it (when loaded). It is
// how `pill serve` brings up a service-managed router, and how a changed
// models.ini is picked up: the router has no reload endpoint.
func (s *Service) Start(ctx context.Context, restart bool) error {
	plist, err := s.PlistPath()
	if err != nil {
		return err
	}
	if !s.Loaded(ctx) {
		return s.run(ctx, "bootstrap", s.domain(), plist)
	}
	if restart {
		return s.run(ctx, "kickstart", "-k", s.target())
	}
	return s.run(ctx, "kickstart", s.target())
}

// Stop boots the job out of launchd, which stops the router and keeps it
// stopped until the next login or `pill serve`.
func (s *Service) Stop(ctx context.Context) error {
	if !s.Loaded(ctx) {
		return nil
	}
	if err := s.run(ctx, "bootout", s.target()); err != nil {
		return err
	}
	// bootout returns before the process is gone; give the port time to free.
	time.Sleep(200 * time.Millisecond)
	return nil
}
