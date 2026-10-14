package cmd

import (
	"bytes"
	"strings"
	"testing"
)

// run executes the CLI against a fresh data dir (never the real home) and
// returns exit code, stdout and stderr.
func run(t *testing.T, home string, args ...string) (int, string, string) {
	t.Helper()
	t.Setenv("NOTES_HOME", home)
	t.Setenv("NOTES_NOW", "2026-01-02T03:04:05Z")
	var out, errb bytes.Buffer
	code := Execute(args, &out, &errb)
	return code, out.String(), errb.String()
}

func TestAddAndLs(t *testing.T) {
	tests := []struct {
		name     string
		commands [][]string
		wantOut  string
	}{
		{"empty", [][]string{{"ls"}}, "no notes\n"},
		{"one note", [][]string{{"add", "buy milk"}, {"ls"}}, "added #1\n#1 buy milk\n"},
		{"tags", [][]string{{"add", "x", "--tag", "a", "--tag", "b"}, {"ls"}}, "added #1\n#1 x [a,b]\n"},
		{"filter", [][]string{{"add", "x", "--tag", "a"}, {"add", "y"}, {"ls", "--tag", "a"}}, "added #1\nadded #2\n#1 x [a]\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			var got strings.Builder
			for _, c := range tc.commands {
				code, out, errs := run(t, home, c...)
				if code != 0 {
					t.Fatalf("%v: exit %d, stderr %q", c, code, errs)
				}
				got.WriteString(out)
			}
			if got.String() != tc.wantOut {
				t.Errorf("output = %q, want %q", got.String(), tc.wantOut)
			}
		})
	}
}

func TestLsJSON(t *testing.T) {
	home := t.TempDir()
	run(t, home, "add", "x", "--tag", "a")
	code, out, _ := run(t, home, "ls", "--format", "json")
	want := `{"notes":[{"id":1,"text":"x","tags":["a"],"created":"2026-01-02T03:04:05Z"}]}` + "\n"
	if code != 0 || out != want {
		t.Errorf("got code %d out %q, want %q", code, out, want)
	}
}

func TestExitCodes(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want int
	}{
		{"unknown command", []string{"nope"}, 2},
		{"unknown flag", []string{"ls", "--nope"}, 2},
		{"bad format", []string{"ls", "--format", "xml"}, 2},
		{"missing text", []string{"add"}, 2},
		{"blank text", []string{"add", "   "}, 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			code, out, errs := run(t, t.TempDir(), tc.args...)
			if code != tc.want {
				t.Errorf("exit = %d, want %d (stderr %q)", code, tc.want, errs)
			}
			if out != "" || !strings.HasPrefix(errs, "error: ") {
				t.Errorf("failure must print nothing to stdout and 'error: ...' to stderr; got %q / %q", out, errs)
			}
		})
	}
}
