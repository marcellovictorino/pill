package output

import (
	"os"
	"strings"
)

// The logo is a small ASCII pill: the left half is plain, the right half blue.
// Each row is split at the pill's seam so the two halves can be coloured
// independently.
var logoRows = [][2]string{
	{`  .-------`, `-------.  `},
	{` (        `, `#######  ) `},
	{`  '-------`, `-------'  `},
}

const (
	ansiBlue  = "\x1b[38;5;33m"
	ansiReset = "\x1b[0m"
)

// Logo renders the pill. With colour off it is plain ASCII.
func Logo(color bool) string {
	var b strings.Builder
	for _, row := range logoRows {
		b.WriteString(row[0])
		if color {
			b.WriteString(ansiBlue + row[1] + ansiReset)
		} else {
			b.WriteString(row[1])
		}
		b.WriteString("\n")
	}
	return b.String()
}

// agentEnvPrefixes lists environment variables that signal "an AI agent is
// driving this process". Agents pay per token, so they never get the logo.
var agentEnvPrefixes = []string{"CLAUDECODE", "CLAUDE_CODE", "PI_", "CODEX_", "OPENCODE", "CURSOR_AGENT", "GEMINI_CLI"}

// RunByAgent reports whether the environment looks like an agent session.
func RunByAgent() bool {
	for _, kv := range os.Environ() {
		for _, p := range agentEnvPrefixes {
			if strings.HasPrefix(kv, p) {
				return true
			}
		}
	}
	return false
}
