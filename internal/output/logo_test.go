package output

import "testing"

func TestRunByAgent(t *testing.T) {
	cases := []struct {
		name    string
		environ []string
		want    bool
	}{
		{"plain terminal", []string{"HOME=/h", "TERM=xterm-256color"}, false},
		{"Pi settings exported from a shell profile", []string{"PI_SKIP_VERSION_CHECK=1", "PI_CODING_AGENT_DIR=/x"}, false},
		{"Claude Code settings exported from a shell profile", []string{"CLAUDE_CODE_AUTO_COMPAT_WINDOW=300000", "CLAUDE_CODE_DISABLE_FEEDBACK_SURVEY=1"}, false},
		{"Codex home is a setting, not a session", []string{"CODEX_HOME=/x"}, false},
		{"Claude Code session", []string{"CLAUDECODE=1"}, true},
		{"Claude Code entrypoint", []string{"CLAUDE_CODE_ENTRYPOINT=cli"}, true},
		{"generic agent marker", []string{"AI_AGENT=claude-code"}, true},
		{"Codex sandbox", []string{"CODEX_SANDBOX_NETWORK_DISABLED=1"}, true},
		{"a value is not a name", []string{"EDITOR=CLAUDECODE"}, false},
	}
	for _, c := range cases {
		if got := runByAgent(c.environ); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}
