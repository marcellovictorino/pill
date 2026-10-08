package service

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"testing"
)

var update = os.Getenv("UPDATE_GOLDEN") != ""

func TestPlistGolden(t *testing.T) {
	got := Plist("/opt/homebrew/bin/llama-server",
		[]string{"--models-dir", "/h/.pill/models", "--models-preset", "/h/.pill/models.ini", "--port", "11435"},
		"/h/.pill", "/h/.pill/logs/server.log", "/opt/homebrew/bin:/usr/bin")
	path := filepath.Join("testdata", "plist.golden")
	if update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run UPDATE_GOLDEN=1 go test ./internal/service)", err)
	}
	if string(got) != string(want) {
		t.Errorf("plist differs from golden:\n%s", got)
	}
}

func TestPlistIsWellFormedAndEscaped(t *testing.T) {
	got := Plist("/bin/x", []string{"--name", "a&b <c>"}, "/w", "/l", "/p")
	if err := xml.Unmarshal(got, new(struct{})); err != nil {
		t.Fatalf("not well-formed XML: %v\n%s", err, got)
	}
}
