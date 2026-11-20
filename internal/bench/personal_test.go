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

// A package upgrade or an edited skill outside the Pi folder changes what the
// agent does, so it must change the setup hash too.
func TestSetupHashCoversPackagesAndExternalSkills(t *testing.T) {
	d, ext := t.TempDir(), t.TempDir()
	write(t, ext, "SKILL.md", "one")
	write(t, d, "settings.json", `{"skills":["`+ext+`"]}`)
	write(t, d, "npm/package-lock.json", "v1")
	base := SetupHash(d)
	write(t, ext, "SKILL.md", "two")
	afterSkill := SetupHash(d)
	if afterSkill == base {
		t.Error("an edited external skill must change the setup hash")
	}
	write(t, d, "npm/package-lock.json", "v2")
	if SetupHash(d) == afterSkill {
		t.Error("an upgraded package must change the setup hash")
	}
}

// An entry that cannot be read must stop the run, not shrink the setup quietly;
// only a link whose target is gone is skipped.
func TestCopyPersonalPiSkipsOnlyDanglingLinks(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root can read everything")
	}
	src := t.TempDir()
	write(t, src, "AGENTS.md", "rules")
	if err := os.Symlink(filepath.Join(src, "gone"), filepath.Join(src, "dangling")); err != nil {
		t.Fatal(err)
	}
	if err := CopyPersonalPi(src, filepath.Join(t.TempDir(), "copy")); err != nil {
		t.Fatalf("a dangling link is skipped: %v", err)
	}

	locked := t.TempDir()
	write(t, locked, "secret.md", "x")
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })
	if err := os.Symlink(filepath.Join(locked, "secret.md"), filepath.Join(src, "skill.md")); err != nil {
		t.Fatal(err)
	}
	if err := CopyPersonalPi(src, filepath.Join(t.TempDir(), "copy2")); err == nil {
		t.Error("a link that cannot be resolved must fail the copy")
	}
}

// The Hindsight skill tells the agent to use the memory service on this computer,
// so a personal run must leave it out as it leaves out the add-on.
func TestCopyPersonalPiLeavesOutHindsightSkills(t *testing.T) {
	src := t.TempDir()
	write(t, src, "skills/one/SKILL.md", "skill")
	write(t, src, "skills/hindsight-memory/SKILL.md", "memory service")
	dest := filepath.Join(t.TempDir(), "copy")
	if err := CopyPersonalPi(src, dest); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dest, "skills", "one", "SKILL.md")); err != nil {
		t.Errorf("an ordinary skill must be copied: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "skills", "hindsight-memory")); !os.IsNotExist(err) {
		t.Errorf("the Hindsight skill must stay out of a personal run, stat error: %v", err)
	}
}

// A relative reference in settings.json is relative to the Pi folder, not to the
// folder pill was started from: editing it must change the hash, and an
// unrelated folder of the same name in the working directory must not.
func TestSetupHashResolvesRelativeReferencesAgainstThePiFolder(t *testing.T) {
	d, cwd := t.TempDir(), t.TempDir()
	t.Chdir(cwd)
	write(t, d, "settings.json", `{"skills":["./custom-skills"]}`)
	write(t, d, "custom-skills/SKILL.md", "one")
	base := SetupHash(d)
	write(t, cwd, "custom-skills/SKILL.md", "unrelated")
	if SetupHash(d) != base {
		t.Error("a folder in the working directory must not change the hash")
	}
	write(t, d, "custom-skills/SKILL.md", "two")
	if SetupHash(d) == base {
		t.Error("an edit under the Pi folder's relative reference must change the hash")
	}
}
