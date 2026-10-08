package registry

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/marcellovictorino/pill/internal/config"
)

// newRegistry opens a registry whose every path lives in a temp directory.
func newRegistry(t *testing.T) *Registry {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", filepath.Join(dir, "home"))
	t.Setenv("PILL_HOME", filepath.Join(dir, "pill"))
	t.Setenv("PI_CODING_AGENT_DIR", filepath.Join(dir, "piagent"))
	t.Setenv("XDG_CONFIG_HOME", "")
	p, err := config.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	s, err := config.Load(p)
	if err != nil {
		t.Fatal(err)
	}
	r, err := Open(p, s)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// A catalog model referenced through Hugging Face must be the same entry,
// with the same settings, whether or not its file has been downloaded: a
// benchmark that downloads first must measure what a later `pill add` of the
// same reference registers.
func TestCatalogReferenceResolvesIdenticallyBeforeAndAfterDownload(t *testing.T) {
	r := newRegistry(t)
	entry := r.Catalog.Entries()[0]
	ref := "hf.co/" + entry.Family.Repo + ":" + entry.Variant.Quant

	before, err := r.Resolve(&Snapshot{Models: &config.ModelsFile{}, Results: &config.Results{Files: map[string]config.FileInfo{}}}, ref, 0)
	if err != nil {
		t.Fatal(err)
	}
	if before.Name != entry.Name || before.Ctx != entry.Ctx {
		t.Fatalf("before download: %s ctx %d, want the catalog entry %s ctx %d", before.Name, before.Ctx, entry.Name, entry.Ctx)
	}

	if err := os.MkdirAll(r.Settings.ModelsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(r.Settings.ModelsDir, entry.Variant.File), []byte("gguf"), 0o644); err != nil {
		t.Fatal(err)
	}
	snap := &Snapshot{Models: &config.ModelsFile{}, Results: &config.Results{Files: map[string]config.FileInfo{
		entry.Variant.File: {Repo: entry.Family.Repo, Quant: entry.Variant.Quant},
	}}}
	after, err := r.Resolve(snap, ref, 0)
	if err != nil {
		t.Fatal(err)
	}
	if after.File = ""; !reflect.DeepEqual(withoutFile(before), withoutFile(after)) {
		t.Errorf("the entry changed once the file was downloaded:\n before %+v\n after  %+v", before, after)
	}
}

func withoutFile(m config.Model) config.Model { m.File = ""; return m }

// A quantisation suffix on some other repository's file proves nothing.
func TestLocalFileForQuantNeedsADownloadRecordFromTheSameRepository(t *testing.T) {
	r := newRegistry(t)
	if err := os.MkdirAll(r.Settings.ModelsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(r.Settings.ModelsDir, "a-Q4_K_M.gguf"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	snap := &Snapshot{Models: &config.ModelsFile{}, Results: &config.Results{Files: map[string]config.FileInfo{
		"a-Q4_K_M.gguf": {Repo: "a/x", Quant: "Q4_K_M"},
	}}}
	m, err := r.Resolve(snap, "b/y:Q4_K_M", 0)
	if err != nil {
		t.Fatal(err)
	}
	if m.File != "" {
		t.Errorf("repository b/y was given %q, which came from a/x", m.File)
	}
	m, _ = r.Resolve(snap, "a/x:Q4_K_M", 0)
	if m.File != "a-Q4_K_M.gguf" {
		t.Errorf("a/x lost its own download: %q", m.File)
	}
}

// Update must serialise writers: concurrent increments never lose one.
func TestUpdateSerialisesConcurrentWriters(t *testing.T) {
	r := newRegistry(t)
	const writers = 8
	done := make(chan error, writers)
	for i := 0; i < writers; i++ {
		name := string(rune('a' + i))
		go func() {
			_, err := r.Update(t.Context(), func(s *Snapshot) error {
				s.Models.Upsert(config.Model{Name: name, File: name + ".gguf", Ctx: 4096})
				return nil
			})
			done <- err
		}()
	}
	for i := 0; i < writers; i++ {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
	snap, err := r.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Models.Models) != writers {
		t.Errorf("%d of %d concurrent additions survived", len(snap.Models.Models), writers)
	}
}
