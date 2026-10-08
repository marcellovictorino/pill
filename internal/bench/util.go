package bench

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
)

func contains(haystack, needle string) bool { return strings.Contains(haystack, needle) }

func truncate(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// lastLine returns the last non-empty line of b (the verifier prints its JSON last).
func lastLine(b []byte) []byte {
	lines := bytes.Split(bytes.TrimSpace(b), []byte("\n"))
	return lines[len(lines)-1]
}

// saveJSON writes v as indented JSON next to the benchmark results; failures
// are ignored because these files are diagnostics, not results.
func saveJSON(path string, v any) {
	if data, err := json.MarshalIndent(v, "", "  "); err == nil {
		_ = os.WriteFile(path, append(data, '\n'), 0o644)
	}
}
