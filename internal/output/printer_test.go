package output

import (
	"bytes"
	"strings"
	"testing"
)

func newTestPrinter(t *testing.T, format string, tty bool) (*Printer, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	var out, errOut bytes.Buffer
	p, err := NewPrinter(&out, &errOut, format, tty)
	if err != nil {
		t.Fatal(err)
	}
	return p, &out, &errOut
}

func TestAutoFormatFollowsTTY(t *testing.T) {
	if p, _, _ := newTestPrinter(t, "", true); !p.Human {
		t.Error("terminal should default to human output")
	}
	if p, _, _ := newTestPrinter(t, "", false); p.Human {
		t.Error("a pipe should default to TOON")
	}
	if p, _, _ := newTestPrinter(t, "toon", true); p.Human {
		t.Error("--format toon must override the terminal")
	}
	if _, err := NewPrinter(nil, nil, "yaml", false); err == nil {
		t.Error("unknown format should be rejected")
	}
}

func TestEmitTOONWithHelp(t *testing.T) {
	p, out, _ := newTestPrinter(t, "", false)
	p.Emit(Obj{}.Set("router", "down").Set("help", []string{"run `pill serve`", "or: pill"}))
	want := "router: down\nhelp[1]: run `pill serve`\nhelp[2]: or: pill\n"
	if out.String() != want {
		t.Errorf("got %q, want %q", out.String(), want)
	}
}

func TestFailGoesToStdoutInTOONAndStderrForHumans(t *testing.T) {
	e := Fail([]string{"try again"}, "boom: %d", 1)

	p, out, errOut := newTestPrinter(t, "", false)
	p.Fail(e)
	if got, want := out.String(), "error: boom: 1\nhelp[1]: try again\n"; got != want {
		t.Errorf("toon: got %q, want %q", got, want)
	}
	if errOut.Len() != 0 {
		t.Errorf("toon mode wrote to stderr: %q", errOut.String())
	}

	p, out, errOut = newTestPrinter(t, "human", false)
	p.Fail(e)
	if out.Len() != 0 || !strings.Contains(errOut.String(), "error: boom: 1") {
		t.Errorf("human mode: stdout=%q stderr=%q", out.String(), errOut.String())
	}
}

func TestColourRespectsNoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	if p, _, _ := newTestPrinter(t, "", true); !p.Color {
		t.Error("terminal without NO_COLOR should get colour")
	}
	t.Setenv("NO_COLOR", "1")
	if p, _, _ := newTestPrinter(t, "", true); p.Color || p.ShowLogo() {
		t.Error("NO_COLOR must disable colour and the logo")
	}
}

func TestLogoHiddenFromAgents(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("CLAUDECODE", "")
	p, _, _ := newTestPrinter(t, "", true)
	// Setenv("CLAUDECODE","") leaves the variable defined, which is what agents do.
	if p.ShowLogo() {
		t.Error("logo must not show when CLAUDECODE is set")
	}
}

func TestLogoShape(t *testing.T) {
	plain := Logo(false)
	if strings.Contains(plain, "\x1b") {
		t.Error("plain logo must not contain escape codes")
	}
	if c := Logo(true); !strings.Contains(c, ansiBlue) || !strings.Contains(c, ansiRedBg) {
		t.Error("coloured logo should have a blue half and a red block")
	}
	if plain != "(pi|ll)\n" {
		t.Errorf("the logo is one row, as wide as the word plus its capsule ends: %q", plain)
	}
}

func TestHumanTable(t *testing.T) {
	p, out, _ := newTestPrinter(t, "human", false)
	p.Emit(Obj{}.Set("models", []Obj{
		Obj{}.Set("name", "a").Set("size", "1 GB"),
		Obj{}.Set("name", "longer").Set("size", "2 GB"),
	}))
	got := out.String()
	for _, want := range []string{"NAME", "SIZE", "longer", "2 GB"} {
		if !strings.Contains(got, want) {
			t.Errorf("table missing %q:\n%s", want, got)
		}
	}
}
