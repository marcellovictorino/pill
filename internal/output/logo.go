package output

import (
	"os"
	"strings"
)

// The logo is the word "pill" drawn as a capsule on a single row: round
// brackets for the ends and a seam in the middle. The left half is plain blue
// text; the right half sits on a red block. Plain ASCII, so it works in any
// font. (Baseline picked by the owner; expect further iteration.)
const (
	logoLeft  = "(pi|"
	logoRight = "ll"
	logoEnd   = ")"
)

const (
	ansiBlue  = "\x1b[38;5;33m"
	ansiRedBg = "\x1b[48;5;203m\x1b[38;5;16m" // dark text on a red block
	ansiReset = "\x1b[0m"
)

// Logo renders the pill. With colour off the shape is the same, uncoloured.
func Logo(color bool) string {
	if !color {
		return logoLeft + logoRight + logoEnd + "\n"
	}
	return ansiBlue + logoLeft + ansiReset + ansiRedBg + logoRight + ansiReset + ansiBlue + logoEnd + ansiReset + "\n"
}

// agentEnvNames are variables that a running agent session sets, matched by
// exact name. Agents pay per token, so they never get the logo.
//
// Names are exact on purpose. Prefixes such as "CLAUDE_CODE" or "PI_" also
// match settings people export from their shell profile
// (CLAUDE_CODE_DISABLE_FEEDBACK_SURVEY, PI_SKIP_VERSION_CHECK) and would hide
// the logo from a human. An agent's own bash tool is not a terminal anyway, so
// the TTY check already keeps it out of agent output; this list only matters
// when an agent allocates a pseudo-terminal.
var agentEnvNames = map[string]bool{
	"CLAUDECODE": true, "CLAUDE_CODE_ENTRYPOINT": true, "CLAUDE_CODE_SESSION_ID": true,
	"AI_AGENT": true, "OPENCODE": true, "CURSOR_AGENT": true, "GEMINI_CLI": true,
}

// RunByAgent reports whether the environment looks like an agent session.
func RunByAgent() bool { return runByAgent(os.Environ()) }

// runByAgent is RunByAgent over an explicit environment ("NAME=value" lines),
// which keeps it testable whatever shell the tests run from.
func runByAgent(environ []string) bool {
	for _, kv := range environ {
		name, _, _ := strings.Cut(kv, "=")
		// Codex marks its sandboxed commands with CODEX_SANDBOX*.
		if agentEnvNames[name] || strings.HasPrefix(name, "CODEX_SANDBOX") {
			return true
		}
	}
	return false
}
