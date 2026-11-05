package version

import "testing"

// The Homebrew release plan requires source-archive builds to show their commit.
func TestCommitUsesExplicitBuildStamp(t *testing.T) {
	old := buildCommit
	t.Cleanup(func() { buildCommit = old })
	// The source commit that introduced the latest catalog measurements.
	buildCommit = "f190f88321df47733364cb19c67e179d12400987"
	if got := Commit(); got != "f190f88" {
		t.Fatalf("Commit() = %q, want stamped short commit", got)
	}
}
