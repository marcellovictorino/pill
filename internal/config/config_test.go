package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func tempPaths(t *testing.T) Paths {
	t.Helper()
	root := t.TempDir()
	return Paths{ConfigDir: filepath.Join(root, "config"), Root: filepath.Join(root, "root")}
}

func TestResolveHonoursPillHomeAndXDG(t *testing.T) {
	t.Setenv("PILL_HOME", "/x/pill")
	p, err := Resolve()
	if err != nil || p.Root != "/x/pill" || p.ConfigDir != "/x/pill/config" {
		t.Fatalf("PILL_HOME: %+v, %v", p, err)
	}

	t.Setenv("PILL_HOME", "")
	t.Setenv("HOME", "/h")
	t.Setenv("XDG_CONFIG_HOME", "/xdg")
	p, _ = Resolve()
	if p.ConfigDir != "/xdg/pill" || p.Root != "/h/.pill" {
		t.Errorf("XDG: %+v", p)
	}

	t.Setenv("XDG_CONFIG_HOME", "")
	p, _ = Resolve()
	if p.ConfigDir != "/h/.config/pill" {
		t.Errorf("default config dir: %+v", p)
	}
}

func TestLoadDefaultsAndOverrides(t *testing.T) {
	p := tempPaths(t)
	t.Setenv("PILL_PORT", "")
	s, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if s.Port != DefaultPort || s.IdleSeconds != DefaultIdleSeconds || s.ModelsDir != p.DefaultModelsDir() {
		t.Errorf("defaults wrong: %+v", s)
	}

	if err := os.MkdirAll(p.ConfigDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "port = 12000\nidle_seconds = 60\ndefault = \"m\"\nmodels_dir = \"/models\"\n"
	if err := os.WriteFile(p.ConfigFile(), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	s, _ = Load(p)
	if s.Port != 12000 || s.IdleSeconds != 60 || s.Default != "m" || s.ModelsDir != "/models" {
		t.Errorf("file values wrong: %+v", s)
	}

	t.Setenv("PILL_PORT", "13000")
	if s, _ = Load(p); s.Port != 13000 {
		t.Errorf("PILL_PORT should win, got %d", s.Port)
	}
	t.Setenv("PILL_PORT", "banana")
	if _, err := Load(p); err == nil {
		t.Error("an invalid PILL_PORT must be an error")
	}
}

func TestSetDefaultKeepsComments(t *testing.T) {
	p := tempPaths(t)
	if err := SetDefault(p, "one"); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(p.ConfigFile())
	if !strings.Contains(string(data), `default = "one"`) {
		t.Fatalf("not written: %s", data)
	}

	custom := "# my note\nport = 12000\ndefault = \"one\"\n# trailing\n"
	_ = os.WriteFile(p.ConfigFile(), []byte(custom), 0o644)
	if err := SetDefault(p, "two"); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(p.ConfigFile())
	want := "# my note\nport = 12000\ndefault = \"two\"\n# trailing\n"
	if string(data) != want {
		t.Errorf("got %q, want %q", data, want)
	}

	_ = os.WriteFile(p.ConfigFile(), []byte("port = 1\n"), 0o644)
	_ = SetDefault(p, "three")
	data, _ = os.ReadFile(p.ConfigFile())
	if string(data) != "default = \"three\"\nport = 1\n" {
		t.Errorf("missing default should be prepended: %q", data)
	}
}

func TestModelsRoundTrip(t *testing.T) {
	p := tempPaths(t)
	mf, err := LoadModels(p)
	if err != nil || len(mf.Models) != 0 {
		t.Fatalf("missing file should be empty: %v %v", mf, err)
	}
	mf.Upsert(Model{Name: "b", File: "b.gguf", Ctx: 8192, MaxTokens: 2048, Preset: map[string]any{"temp": 0.7}})
	mf.Upsert(Model{Name: "a", File: "a.gguf", Ctx: 4096})
	mf.Upsert(Model{Name: "b", File: "b2.gguf", Ctx: 8192}) // replace
	if err := SaveModels(p, mf); err != nil {
		t.Fatal(err)
	}
	got, err := LoadModels(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Models) != 2 || got.Models[0].Name != "a" || got.Models[1].File != "b2.gguf" {
		t.Errorf("round trip wrong: %+v", got.Models)
	}
	if !got.Remove("a") || got.Remove("a") {
		t.Error("Remove should report whether the entry existed")
	}
}

func TestWriteFileAtomicFollowsSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "dotfiles", "models.toml")
	link := filepath.Join(dir, "config", "models.toml")
	_ = os.MkdirAll(filepath.Dir(target), 0o755)
	_ = os.MkdirAll(filepath.Dir(link), 0o755)
	_ = os.WriteFile(target, []byte("old"), 0o644)
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if err := WriteFileAtomic(link, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	if st, _ := os.Lstat(link); st.Mode()&os.ModeSymlink == 0 {
		t.Error("the symlink was replaced by a regular file")
	}
	if data, _ := os.ReadFile(target); string(data) != "new" {
		t.Errorf("target not updated: %q", data)
	}
}

func TestEffectiveState(t *testing.T) {
	passed := &ModelState{State: StatePassed, PresetHash: "h1", GGUFSize: 100}
	cases := []struct {
		name string
		st   *ModelState
		hash string
		size int64
		want string
	}{
		{"no state", nil, "h1", 100, StateUnverified},
		{"passed and matching", passed, "h1", 100, StatePassed},
		{"preset changed", passed, "h2", 100, StateUnverified},
		{"file changed", passed, "h1", 200, StateUnverified},
		{"failed stays failed", &ModelState{State: StateFailed, PresetHash: "h1"}, "h1", 0, StateFailed},
		{"hash unknown yet", &ModelState{State: StatePassed}, "h2", 5, StatePassed},
	}
	for _, c := range cases {
		if got := EffectiveState(c.st, c.hash, c.size); got != c.want {
			t.Errorf("%s: got %s, want %s", c.name, got, c.want)
		}
	}
}

func TestResultsRoundTrip(t *testing.T) {
	p := tempPaths(t)
	r, err := LoadResults(p)
	if err != nil {
		t.Fatal(err)
	}
	r.Ensure("m").InPi = true
	r.Files["f.gguf"] = FileInfo{Repo: "o/r", Quant: "Q4", Size: 7}
	if err := SaveResults(p, r); err != nil {
		t.Fatal(err)
	}
	got, err := LoadResults(p)
	if err != nil || !got.Get("m").InPi || got.Files["f.gguf"].Repo != "o/r" {
		t.Errorf("round trip wrong: %+v %v", got, err)
	}
}
