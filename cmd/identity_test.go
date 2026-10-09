package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/marcellovictorino/pill/internal/testutil"
)

// twoRepoHub has two repositories that offer a file with the same name (equal
// size in c/z) plus a nested path and a repository with a different file name.
func twoRepoHub(t *testing.T) *testutil.Hub {
	t.Helper()
	return testutil.NewHub(t, map[string]map[string][]byte{
		"a/x": {"model-Q4_K_M.gguf": []byte("AAAA-from-a")},
		"b/y": {"model-Q4_K_M.gguf": []byte("BBBBBBBB-from-b-longer"), "y-Q8_0.gguf": []byte("y-q8")},
		"c/z": {"model-Q4_K_M.gguf": []byte("CCCC-from-c")}, // same size as a/x's
		"n/s": {"sub/dir/nested-Q4_K_M.gguf": []byte("nested-weights")},
		// Two folders with the same file name: a bare name is ambiguous.
		"d/w": {"one/m-Q4_K_M.gguf": []byte("one-one"), "two/m-Q4_K_M.gguf": []byte("two-two-two")},
	})
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// The models directory is flat, so two repositories can offer the same file
// name. The second pull must neither reuse the first's bytes (and record the
// wrong checksum) nor overwrite them.
func TestPullRefusesAFileNameThatBelongsToAnotherRepository(t *testing.T) {
	e := newEnv(t)
	hub := twoRepoHub(t)
	e.run("pull", "a/x/model-Q4_K_M.gguf").ok(t)
	path := filepath.Join(e.modelsDir(), "model-Q4_K_M.gguf")
	want := readFile(t, path)

	for _, other := range []string{"b/y/model-Q4_K_M.gguf", "c/z/model-Q4_K_M.gguf"} { // different and equal size
		hits := hub.Hits("/resolve/")
		e.run("pull", other).fails(t, 1, "is from a/x")
		if hub.Hits("/resolve/") != hits {
			t.Errorf("%s: nothing should be downloaded", other)
		}
		if got := readFile(t, path); got != want {
			t.Errorf("%s overwrote the first repository's file: %q", other, got)
		}
	}
	// The same repository again is still a no-op.
	if r := e.run("pull", "a/x/model-Q4_K_M.gguf").ok(t); !strings.Contains(r.out, "already present") {
		t.Errorf("repeat pull:\n%s", r.out)
	}
}

// A file copied in by hand has no record; if it has the right size but not the
// right content, pull must say so instead of trusting the size.
func TestPullChecksAnUnrecordedFileAgainstHuggingFace(t *testing.T) {
	e := newEnv(t)
	twoRepoHub(t)
	if err := os.MkdirAll(e.modelsDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	same := filepath.Join(e.modelsDir(), "model-Q4_K_M.gguf")
	if err := os.WriteFile(same, []byte("XXXX-not-from-a"), 0o644); err != nil { // 11 bytes, like a/x's
		t.Fatal(err)
	}
	e.run("pull", "a/x/model-Q4_K_M.gguf").fails(t, 1, "unknown source")

	if err := os.WriteFile(same, []byte("AAAA-from-a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if r := e.run("pull", "a/x/model-Q4_K_M.gguf").ok(t); !strings.Contains(r.out, "already present") {
		t.Errorf("the genuine file should be recognised:\n%s", r.out)
	}
	// A file with another size and no record is not replaced either.
	if err := os.Remove(filepath.Join(e.dir, "pill", "results.json")); err != nil { // forget where it came from
		t.Fatal(err)
	}
	if err := os.WriteFile(same, []byte("short"), 0o644); err != nil {
		t.Fatal(err)
	}
	e.run("pull", "b/y/model-Q4_K_M.gguf").fails(t, 1, "unknown source")
	if got := readFile(t, same); got != "short" {
		t.Errorf("an unrecorded file was overwritten: %q", got)
	}
}

// Repository A's a-Q4_K_M.gguf must never be mistaken for repository B's Q4_K_M.
func TestAddDoesNotBorrowAnotherRepositorysFileByQuantisation(t *testing.T) {
	e := newEnv(t)
	twoRepoHub(t)
	e.run("pull", "a/x:Q4_K_M").ok(t)
	e.run("add", "hf.co/b/y:Q4_K_M", "--unverified").fails(t, 1, "pill pull")
	if _, err := os.Stat(filepath.Join(e.dir, "pill", "config", "models.toml")); err == nil && strings.Contains(readFile(t, filepath.Join(e.dir, "pill", "config", "models.toml")), "b/y") {
		t.Error("nothing should be registered for repository b/y")
	}
	// The repository whose download it really is still resolves.
	e.run("add", "hf.co/a/x:Q4_K_M", "--unverified").ok(t)
}

// owner/repo/sub/dir/file.gguf downloads to <models>/file.gguf; the suggested
// `pill add` must find it there.
func TestNestedHuggingFacePathIsRegisteredByItsLocalName(t *testing.T) {
	e := newEnv(t)
	twoRepoHub(t)
	e.run("pull", "n/s/sub/dir/nested-Q4_K_M.gguf").ok(t)
	if _, err := os.Stat(filepath.Join(e.modelsDir(), "nested-Q4_K_M.gguf")); err != nil {
		t.Fatalf("expected the base name in the models dir: %v", err)
	}
	r := e.run("add", "n/s/sub/dir/nested-Q4_K_M.gguf", "--unverified").ok(t)
	if !strings.Contains(r.out, "status: registered") {
		t.Errorf("add:\n%s", r.out)
	}
	toml := readFile(t, filepath.Join(e.dir, "pill", "config", "models.toml"))
	if !strings.Contains(toml, `file = "nested-Q4_K_M.gguf"`) || strings.Contains(toml, "sub/dir") {
		t.Errorf("models.toml:\n%s", toml)
	}
}

// Registering a downloaded file by its name keeps the Hugging Face source, so
// models.toml works on another machine.
func TestAddByFileNameKeepsTheRecordedSource(t *testing.T) {
	e := newEnv(t)
	twoRepoHub(t)
	e.run("pull", "b/y:Q8_0").ok(t)
	e.run("add", "y-Q8_0.gguf", "--unverified").ok(t)
	toml := readFile(t, filepath.Join(e.dir, "pill", "config", "models.toml"))
	if !strings.Contains(toml, `repo = "b/y"`) {
		t.Errorf("the source repository was lost:\n%s", toml)
	}
}

// A catalog model referenced through Hugging Face is the catalog entry, with
// the catalog's settings, before and after it is downloaded.
func TestCatalogModelByHuggingFaceReferenceGetsCatalogIdentity(t *testing.T) {
	e := newEnv(t)
	gemmaHub(t)
	ref := "hf.co/" + gemmaRepo + ":IQ4_XS"
	e.run("pull", ref).ok(t)
	r := e.run("add", ref, "--unverified").ok(t)
	if !strings.Contains(r.out, "model: gemma4-26b-iq4xs-64k") || !strings.Contains(r.out, "ctx: 65536") {
		t.Errorf("add by reference:\n%s", r.out)
	}
	if again := e.run("add", "gemma4-26b", "--unverified").ok(t); !strings.Contains(again.out, "already registered") {
		t.Errorf("the catalog name and the reference are one entry:\n%s", again.out)
	}
}

// A benchmark can take twenty minutes. Whatever else changes the model list
// meanwhile must survive the benchmark writing its verdict.
func TestBenchVerdictDoesNotOverwriteConcurrentChanges(t *testing.T) {
	needNode(t)
	e := newEnv(t)
	solveTier2(t)
	t.Setenv("PILL_FAKE_PI_TIER2_SLEEP", "3") // the fake model "thinks" for 3 seconds in Tier 2
	e.gguf(gemmaFile)
	e.gguf("other-Q4_K_M.gguf")
	tomlPath := filepath.Join(e.dir, "pill", "config", "models.toml")

	var wg sync.WaitGroup
	var bench result
	wg.Add(1)
	go func() {
		defer wg.Done()
		bench = e.run("bench", "run", "gemma4-26b-iq4xs-64k")
	}()
	waitFor(t, func() bool { return strings.Contains(readOrEmpty(e.fakes.PiLog), "tic-tac-toe") }) // Tier 2 has started
	e.run("add", "other-Q4_K_M.gguf", "--unverified", "--ctx", "8192").ok(t)                       // while Tier 2 runs
	wg.Wait()
	_ = bench

	toml := readFile(t, tomlPath)
	if !strings.Contains(toml, "other-q4-k-m") && !strings.Contains(toml, "other-Q4_K_M.gguf") {
		t.Errorf("the benchmark's write-back erased the model added meanwhile:\n%s", toml)
	}
	if !strings.Contains(toml, "gemma4-26b-iq4xs-64k") {
		t.Errorf("the benchmarked model should still be registered:\n%s", toml)
	}
}

func TestBenchDoesNotResurrectAModelRemovedMeanwhile(t *testing.T) {
	needNode(t)
	e := newEnv(t)
	solveTier2(t)
	t.Setenv("PILL_FAKE_PI_TIER2_SLEEP", "3")
	e.gguf(gemmaFile)
	tomlPath := filepath.Join(e.dir, "pill", "config", "models.toml")

	var wg sync.WaitGroup
	var bench result
	wg.Add(1)
	go func() {
		defer wg.Done()
		bench = e.run("bench", "run", "gemma4-26b-iq4xs-64k")
	}()
	waitFor(t, func() bool { return strings.Contains(readOrEmpty(e.fakes.PiLog), "tic-tac-toe") }) // Tier 2 has started
	e.run("rm", "gemma4-26b-iq4xs-64k").ok(t)
	wg.Wait()

	if bench.code != 1 || !strings.Contains(bench.out, "was removed while it was being benchmarked") {
		t.Errorf("exit %d:\n%s", bench.code, bench.out)
	}
	if strings.Contains(readOrEmpty(tomlPath), "gemma4-26b-iq4xs-64k") {
		t.Error("a removed model was put back by the benchmark's write-back")
	}
	if ls := e.run("ls").ok(t); strings.Contains(ls.out, "passed") {
		t.Errorf("no verdict may be recorded for a removed model:\n%s", ls.out)
	}
}

func readOrEmpty(path string) string {
	data, _ := os.ReadFile(path)
	return string(data)
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	for i := 0; i < 200; i++ {
		if cond() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("condition not reached in 10 seconds")
}

// Two declarations of one GGUF (the same weights at two context sizes), and a
// GGUF copied in by hand, must all be registered by sync.
func TestSyncRegistersEveryDeclaredModelWhoseFileIsPresent(t *testing.T) {
	e := newEnv(t)
	gemmaHub(t)
	e.run("pull", "gemma4-26b").ok(t)
	e.run("add", "gemma4-26b-iq4xs-64k", "--unverified").ok(t)
	e.run("add", "gemma4-26b-iq4xs-32k", "--unverified").ok(t) // the same file at another context
	e.forgetLocalState()
	if err := os.RemoveAll(filepath.Dir(e.piModels())); err != nil {
		t.Fatal(err)
	}

	e.run("sync").ok(t)
	ls := e.run("ls").ok(t)
	for _, name := range []string{"gemma4-26b-iq4xs-64k,unverified,", "gemma4-26b-iq4xs-32k,unverified,"} {
		if !strings.Contains(ls.out, name) {
			t.Errorf("%s missing from ls:\n%s", name, ls.out)
		}
	}
	prov := readFile(t, e.piModels())
	if !strings.Contains(prov, `"gemma4-26b-iq4xs-64k"`) || !strings.Contains(prov, `"gemma4-26b-iq4xs-32k"`) {
		t.Errorf("both variants must reach Pi:\n%s", prov)
	}
}

func TestSyncRegistersAHandCopiedFileWithoutResults(t *testing.T) {
	e := newEnv(t)
	e.register()
	if err := os.Remove(filepath.Join(e.dir, "pill", "results.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Dir(e.piModels())); err != nil {
		t.Fatal(err)
	}
	e.run("stop", "--all").ok(t)
	e.run("sync").ok(t)
	if prov := readFile(t, e.piModels()); !strings.Contains(prov, "gemma4-26b-iq4xs-64k") {
		t.Errorf("a present file with no results entry never reached Pi:\n%s", prov)
	}
}

// Two pulls of the same file name from different repositories running at the
// same moment: exactly one may write the file, and the manifest must describe
// the bytes that are really there.
func TestConcurrentPullsOfTheSameNameDoNotOverwriteEachOther(t *testing.T) {
	e := newEnv(t)
	hub := twoRepoHub(t)
	hub.Delay = 400 * time.Millisecond
	var wg sync.WaitGroup
	results := make([]result, 2)
	for i, ref := range []string{"a/x/model-Q4_K_M.gguf", "b/y/model-Q4_K_M.gguf"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i] = e.run("pull", ref)
		}()
	}
	wg.Wait()
	ok := 0
	for _, r := range results {
		if r.code == 0 {
			ok++
		} else if !strings.Contains(r.out, "pill will not reuse or overwrite it") {
			t.Errorf("the loser should be refused as a name clash:\n%s", r.out)
		}
	}
	if ok != 1 {
		t.Fatalf("exactly one pull should win, got %d (%v / %v)", ok, results[0].code, results[1].code)
	}
	got := readFile(t, filepath.Join(e.modelsDir(), "model-Q4_K_M.gguf"))
	manifest := readFile(t, filepath.Join(e.dir, "pill", "results.json"))
	wantRepo := map[string]string{"AAAA-from-a": "a/x", "BBBBBBBB-from-b-longer": "b/y"}[got]
	if wantRepo == "" || !strings.Contains(manifest, `"repo": "`+wantRepo+`"`) {
		t.Errorf("file %q is not described by the manifest:\n%s", got, manifest)
	}
}

// An explicit file reference is held to the same repository rule as a
// quantisation reference.
func TestAddByExplicitFileRejectsAnotherRepositorysDownload(t *testing.T) {
	e := newEnv(t)
	twoRepoHub(t)
	e.run("pull", "a/x/model-Q4_K_M.gguf").ok(t)
	e.run("add", "b/y/sub/model-Q4_K_M.gguf", "--unverified").fails(t, 1, "is from a/x")
	e.run("add", "a/x/model-Q4_K_M.gguf", "--unverified").ok(t)
}

// owner/repo/<folder>/file.gguf names one exact file; a bare name that exists
// in several folders is refused instead of picking the first.
func TestPullUsesTheExactRemotePath(t *testing.T) {
	e := newEnv(t)
	twoRepoHub(t)
	e.run("pull", "d/w/m-Q4_K_M.gguf").fails(t, 1, "several files named")
	e.run("pull", "d/w/two/m-Q4_K_M.gguf").ok(t)
	if got := readFile(t, filepath.Join(e.modelsDir(), "m-Q4_K_M.gguf")); got != "two-two-two" {
		t.Errorf("pulled the wrong folder's file: %q", got)
	}
}

// A file pulled by name still answers to owner/repo:QUANT for the same
// repository, and a repeat pull by name keeps the quantisation.
func TestQuantReferenceFindsAFilePulledByName(t *testing.T) {
	e := newEnv(t)
	twoRepoHub(t)
	e.run("pull", "b/y/y-Q8_0.gguf").ok(t)
	e.run("add", "hf.co/b/y:Q8_0", "--unverified").ok(t)
}

// Removing something that does not exist is a no-op: a models.toml kept in
// dotfiles, with comments, must not be rewritten.
func TestRmOfAnUnknownNameLeavesStateFilesAlone(t *testing.T) {
	e := newEnv(t)
	e.register()
	path := filepath.Join(e.dir, "pill", "config", "models.toml")
	hand := readFile(t, path) + "\n# my own note\n"
	if err := os.WriteFile(path, []byte(hand), 0o644); err != nil {
		t.Fatal(err)
	}
	e.run("rm", "no-such-model").ok(t)
	if got := readFile(t, path); got != hand {
		t.Errorf("models.toml was rewritten by a no-op rm:\n%s", got)
	}
}
