package preset

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/marcellovictorino/pill/internal/config"
)

// -update rewrites the golden files: go test ./internal/preset -update
var update = flag.Bool("update", false, "rewrite golden files")

func golden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *update {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Errorf("%s differs from golden (run with -update to accept)\n--- got ---\n%s--- want ---\n%s", name, got, want)
	}
}

var gemma = config.Model{
	Name: "gemma4-26b-iq4xs-64k", File: "gemma-4-26B-A4B-it-UD-IQ4_XS.gguf", Ctx: 65536,
	Preset: map[string]any{"temp": 1.0, "top-p": 0.95, "top-k": int64(64), "min-p": 0.0},
}

func TestRenderGolden(t *testing.T) {
	second := config.Model{
		Name: "gemma4-26b-iq4xs-32k", File: "gemma-4-26B-A4B-it-UD-IQ4_XS.gguf", Ctx: 32768,
		Preset: map[string]any{"temp": 1.0, "zeta": true, "alpha": "x"},
	}
	golden(t, "models.ini.golden", Render([]config.Model{gemma, second}, "/home/u/.pill/models"))
}

func TestRenderEmpty(t *testing.T) {
	golden(t, "empty.ini.golden", Render(nil, "/m"))
}

func TestSectionHashTracksSettings(t *testing.T) {
	a := SectionHash(gemma, "/m")
	if a != SectionHash(gemma, "/m") {
		t.Error("hash must be stable")
	}
	changed := gemma
	changed.Ctx = 32768
	if a == SectionHash(changed, "/m") {
		t.Error("changing the context must change the hash")
	}
	if a == SectionHash(gemma, "/other") {
		t.Error("changing the models dir must change the hash")
	}
}

func TestFloatsKeepDecimalPoint(t *testing.T) {
	if got := formatValue(1.0); got != "1.0" {
		t.Errorf("got %s", got)
	}
	if got := formatValue(0.95); got != "0.95" {
		t.Errorf("got %s", got)
	}
}
