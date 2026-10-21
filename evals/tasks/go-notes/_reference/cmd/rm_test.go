package cmd

import "testing"

func TestRm(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantCode int
		wantOut  string
	}{
		{"removes", []string{"rm", "1"}, 0, "removed #1\n"},
		{"missing is not an error", []string{"rm", "9"}, 0, "not found #9\n"},
		{"argument order and repeats", []string{"rm", "2", "9", "2"}, 0, "removed #2\nnot found #9\nnot found #2\n"},
		{"json", []string{"rm", "1", "9", "--format", "json"}, 0, `{"removed":[1],"missing":[9]}` + "\n"},
		{"no ids", []string{"rm"}, 2, ""},
		{"bad id", []string{"rm", "abc"}, 2, ""},
		{"zero id", []string{"rm", "0"}, 2, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			run(t, home, "add", "one")
			run(t, home, "add", "two")
			code, out, _ := run(t, home, tc.args...)
			if code != tc.wantCode || out != tc.wantOut {
				t.Errorf("code %d out %q, want %d %q", code, out, tc.wantCode, tc.wantOut)
			}
		})
	}
}

func TestRmInvalidIDRemovesNothing(t *testing.T) {
	home := t.TempDir()
	run(t, home, "add", "one")
	run(t, home, "rm", "1", "oops")
	if _, out, _ := run(t, home, "ls"); out != "#1 one\n" {
		t.Errorf("ls after failed rm = %q", out)
	}
}

func TestIDsAreNotReused(t *testing.T) {
	home := t.TempDir()
	run(t, home, "add", "one")
	run(t, home, "add", "two")
	run(t, home, "rm", "2")
	if _, out, _ := run(t, home, "add", "three"); out != "added #3\n" {
		t.Errorf("add after rm = %q, want added #3", out)
	}
}

func TestLsTagIsCaseInsensitive(t *testing.T) {
	home := t.TempDir()
	run(t, home, "add", "x", "--tag", "Ärger")
	if _, out, _ := run(t, home, "ls", "--tag", "ärger"); out != "#1 x [Ärger]\n" {
		t.Errorf("ls = %q", out)
	}
}
