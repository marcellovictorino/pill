package bench

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, root, name, body string) {
	t.Helper()
	p := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// The hash exists so results from a changed setup are told apart later: it
// must move when what steers the agent changes and stay put for history.
func TestSetupHashTracksWhatSteersTheAgent(t *testing.T) {
	d := t.TempDir()
	write(t, d, "AGENTS.md", "rules")
	write(t, d, "skills/a/SKILL.md", "one")
	base := SetupHash(d)
	write(t, d, "sessions/s.jsonl", "history")
	if SetupHash(d) != base {
		t.Error("session history must not change the setup hash")
	}
	write(t, d, "skills/a/SKILL.md", "two")
	if SetupHash(d) == base {
		t.Error("an edited skill must change the setup hash")
	}
}
