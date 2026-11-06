// Package version holds pill's version number and build metadata.
package version

import "runtime/debug"

// Version is bumped by hand with every meaningful feature (see CHANGELOG.md).
// Keep it in sync with the top released section of the changelog.
const Version = "0.6.0"

// buildCommit can be set with -ldflags when building from a source archive.
var buildCommit string

// Commit returns the short git commit the binary was built from.
//
// An explicitly stamped commit takes precedence over Go's VCS metadata.
// It returns "unknown" when neither is available.
func Commit() string {
	if buildCommit != "" {
		return shortCommit(buildCommit)
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	commit, dirty := "", false
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			commit = s.Value
		case "vcs.modified":
			dirty = s.Value == "true"
		}
	}
	if commit == "" {
		return "unknown"
	}
	commit = shortCommit(commit)
	if dirty {
		commit += "-dirty"
	}
	return commit
}

func shortCommit(commit string) string {
	if len(commit) > 7 {
		return commit[:7]
	}
	return commit
}

// String is the one-line form printed by `pill version`.
func String() string {
	return "pill " + Version + " (" + Commit() + ")"
}
