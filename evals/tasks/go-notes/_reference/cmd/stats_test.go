package cmd

import "testing"

func TestStats(t *testing.T) {
	tests := []struct {
		name string
		add  [][]string
		args []string
		want string
	}{
		{"empty human", nil, []string{"stats"}, "total: 0\noldest: -\nnewest: -\ntags: none\n"},
		{"empty json", nil, []string{"stats", "--format", "json"}, `{"total":0,"oldest":null,"newest":null,"tags":[]}` + "\n"},
		{"no tags", [][]string{{"add", "x"}}, []string{"stats"},
			"total: 1\noldest: 2026-01-02T03:04:05Z\nnewest: 2026-01-02T03:04:05Z\ntags: none\n"},
		{"case folded and sorted", [][]string{
			{"add", "a", "--tag", "Work", "--tag", "work"}, {"add", "b", "--tag", "work"}, {"add", "c", "--tag", "home"}, {"add", "d", "--tag", "alpha"},
		}, []string{"stats"},
			"total: 4\noldest: 2026-01-02T03:04:05Z\nnewest: 2026-01-02T03:04:05Z\ntags:\n  work: 2\n  alpha: 1\n  home: 1\n"},
		{"json", [][]string{{"add", "a", "--tag", "x"}}, []string{"stats", "--format", "json"},
			`{"total":1,"oldest":"2026-01-02T03:04:05Z","newest":"2026-01-02T03:04:05Z","tags":[{"tag":"x","count":1}]}` + "\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			for _, a := range tc.add {
				run(t, home, a...)
			}
			code, out, errs := run(t, home, tc.args...)
			if code != 0 || out != tc.want {
				t.Errorf("code %d out %q (stderr %q), want %q", code, out, errs, tc.want)
			}
		})
	}
}

func TestStatsRejectsArguments(t *testing.T) {
	if code, _, _ := run(t, t.TempDir(), "stats", "extra"); code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
}
