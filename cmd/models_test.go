package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/marcellovictorino/pill/internal/pi"
	"github.com/marcellovictorino/pill/internal/testutil"
)

const gemmaRepo = "unsloth/gemma-4-26B-A4B-it-GGUF"

func gemmaHub(t *testing.T) *testutil.Hub {
	t.Helper()
	return testutil.NewHub(t, map[string]map[string][]byte{
		gemmaRepo: {
			gemmaFile:                            []byte("pretend-this-is-13GB-of-IQ4_XS"),
			"gemma-4-26B-A4B-it-UD-Q3_K_XL.gguf": []byte("pretend-q3"),
			"mmproj-F16.gguf":                    []byte("vision"),
		},
		"o/r": {"r-Q4_K_M.gguf": []byte("tiny-model-bytes"), "r-Q8_0.gguf": []byte("tiny-q8")},
	})
}

// Intent (captain): the list is per device, from measurements on that exact
// chip and memory, with explicit empty and unverified states.
func TestCatalogRecommendsPerDevice(t *testing.T) {
	e := newEnv(t)
	r := e.run("catalog", "--format", "agent").ok(t)
	golden(t, "catalog", r.out)

	// A 16 GB Mac: no catalog file fits, so none is offered for download and
	// the footer says how many were left out. Measured runs still show.
	e.sys.RAM = 16 << 30
	r = e.run("catalog", "--format", "agent").ok(t)
	for _, want := range []string{"models[", "catalog models that need more memory than this Mac has: 3", "no catalog model fits"} {
		if !strings.Contains(r.out, want) {
			t.Errorf("16 GB output lacks %q:\n%s", want, r.out)
		}
	}
	if strings.Contains(r.out, "unmeasured") {
		t.Errorf("a model that does not fit must not be listed as untested:\n%s", r.out)
	}

	// Same memory as the tested Mac but another chip: its results are not
	// borrowed. The fitting catalog models list with dashes, nothing is tried.
	e.sys.RAM, e.sys.Chip = 24<<30, "Apple M3 Max"
	r = e.run("catalog", "--format", "agent").ok(t)
	if !strings.Contains(r.out, "models[3]") || !strings.Contains(r.out, `minicpm,Q2,64k,"-","-","-","-","-",`) || strings.Contains(r.out, "22/22") {
		t.Errorf("another chip must show dashes, not the M4 result:\n%s", r.out)
	}

	// Detection failed: explicit state, nothing measured, still exit 0.
	e.sys.Chip, e.sys.ChipErr = "", errors.New("no sysctl")
	r = e.run("catalog", "--format", "agent").ok(t)
	if !strings.Contains(r.out, "chip: unknown") || strings.Contains(r.out, "22/22") || !strings.Contains(r.out, "could not detect") {
		t.Errorf("failed detection must be explicit and measure nothing:\n%s", r.out)
	}
}

// Intent (captain): on a 16 GB Mac, where nothing passes, `pill catalog`
// shows the best that was tried there, closest to a pass first, one row per
// model with its short ID, and never offers a tried model as a download.
func TestCatalogShowsWhatWasTriedOnThisMac(t *testing.T) {
	e := newEnv(t)
	e.sys.RAM, e.sys.Chip = 16<<30, "Apple M2 Pro"
	r := e.run("catalog", "--format", "agent").ok(t)
	golden(t, "catalog_16gb_m2pro", r.out)
	// Qwen3.5 9B (21/22) is closer to a pass than Ornith 1.5 9B (18/22).
	i21, i18 := strings.Index(r.out, "qwen3.5-9b,Q4,32k"), strings.Index(r.out, "ornith1.5-9b,Q6")
	if i21 < 0 || i18 < 0 || i21 > i18 || !strings.Contains(r.out, "21/22") {
		t.Errorf("tried models must be ordered by Tier 2 score:\n%s", r.out)
	}
	// Landing pages group in brackets, so a 9B model that wins them but fails
	// multi-step tasks is visible before downloading it.
	if !strings.Contains(r.out, "15|21|0|2|(90,95,76)") {
		t.Errorf("Tier 3 must read Go|cleaning|insight|Kanban|(nursery,Mars,tax):\n%s", r.out)
	}
	// An offered model that failed here shows even when it ranks below the closest five.
	if !strings.Contains(r.out, "minicpm,Q2,64k,fail (31s),") || !strings.Contains(r.out, ",6%,") {
		t.Errorf("the failed default must show its M2 Pro result:\n%s", r.out)
	}
	// Agents get the source of an offered model; a tried model has none to give.
	if !strings.Contains(r.out, "hf.co/AtomicChat/MiniCPM-V-4.7-35B-A3B-GGUF:AD-IQ2_XS-IQ2_XXS") {
		t.Errorf("an offered row must carry its Hugging Face reference:\n%s", r.out)
	}
	// The row counts follow the cap: the closest five tried models and the offered ones.
	if !strings.Contains(r.out, "models[7]") || !strings.Contains(r.out, "more tried results are in the catalog records") {
		t.Errorf("seven rows and a note on the rest:\n%s", r.out)
	}
}

// Intent (captain): no wrapped lines. Every row of the human list stays under
// 90 characters on the Macs the records cover, and the default is marked *.
func TestCatalogHumanRowsFitUnder90Characters(t *testing.T) {
	for _, dev := range []struct {
		chip string
		gb   int64
	}{{"Apple M4", 24}, {"Apple M2 Pro", 16}} {
		e := newEnv(t)
		e.sys.RAM, e.sys.Chip = dev.gb<<30, dev.chip
		r := e.run("catalog", "--format", "human").ok(t)
		for _, line := range strings.Split(r.out, "\n") {
			if strings.HasPrefix(line, "hint:") {
				continue // hints are sentences, not rows
			}
			if len([]rune(line)) >= 90 {
				t.Errorf("%s: a %d character line would wrap:\n%s", dev.chip, len([]rune(line)), line)
			}
		}
		for _, head := range []string{"ID", "QUANT", "CTX", "TIER 1", "TIER 2", "TIER 3", "TOK/S", "FREE MEM"} {
			if !strings.Contains(r.out, head) {
				t.Errorf("%s: no %s column:\n%s", dev.chip, head, r.out)
			}
		}
		if strings.Contains(r.out, "REASON") || strings.Contains(r.out, "NOTE") {
			t.Errorf("%s: no reason or note column:\n%s", dev.chip, r.out)
		}
	}
}

// Intent (captain): side by side, a model's Tier 1 time and Tier 2 time are
// the point of the table ("it took 48 seconds ... almost 5 minutes"), read as
// "pass (29s)", and a failed run must not read as a pass. gemma4-26b Q3KXL
// passed both tiers but pushed 4.75 GB to swap: it gets no ✓, and agents read
// FAIL. "The red cross should be only for the models marked as fail", so it
// gets no ✗ either. "There should be no relax option": muse-30b-dflash passed
// Tier 1 only with looser limits (111 s against the 60 s cutoff), so it reads
// fail and carries the ✗.
func TestCatalogShowsTimesAndVerdict(t *testing.T) {
	e := newEnv(t)
	human := e.run("catalog", "--format", "human").ok(t).out
	var q3 string
	for _, line := range strings.Split(human, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "gemma4-26b") && strings.Contains(line, "Q3KXL") {
			q3 = line
		}
	}
	if !strings.Contains(q3, "pass (48s)") || !strings.Contains(q3, "22/22 4.9m") {
		t.Errorf("Tier 1 and Tier 2 times must show on the row:\n%s", human)
	}
	if strings.Contains(q3, "✗") || strings.Contains(q3, "✓") {
		t.Errorf("a row whose Tier 1 reads pass but failed on swap carries no mark:\n%s", q3)
	}
	for _, line := range strings.Split(human, "\n") {
		if strings.Contains(line, "✗") != strings.Contains(line, " fail (") {
			t.Errorf("the ✗ belongs on exactly the rows marked fail:\n%s", line)
		}
	}
	if !strings.Contains(human, "minicpm✓") || !strings.Contains(human, "muse-30b-dflash✗") || !strings.Contains(human, "fail (111s)") {
		t.Errorf("a pass carries ✓, and a run over the Tier 1 cutoff fails with ✗:\n%s", human)
	}
	agent := e.run("catalog", "--format", "agent").ok(t).out
	if !strings.Contains(agent, "gemma4-26b,Q3KXL,64k,pass (48s),22/22 4.9m,") || !strings.Contains(agent, ",11%,FAIL,") {
		t.Errorf("agents get the times and a verdict word:\n%s", agent)
	}
	for _, out := range []string{human, agent} {
		if strings.Contains(out, "relax") || strings.Contains(out, "~") {
			t.Errorf("no relaxed verdict, in either format:\n%s", out)
		}
	}
}

// Intent (captain): the catalog reads "more polished. Like it presents a
// report, without underscore": the facts above the table say Memory and
// "24GB RAM", not ram_gb: 24. Agents keep their field names.
func TestCatalogReadsAsAReport(t *testing.T) {
	e := newEnv(t)
	human := e.run("catalog", "--format", "human").ok(t).out
	if !strings.HasPrefix(human, "Chip    Apple M4\nMemory  24GB RAM\n\n") || strings.Contains(human, "_") {
		t.Errorf("the report opens with Chip and Memory and has no underscores:\n%s", human)
	}
	agent := e.run("catalog", "--format", "agent").ok(t).out
	if !strings.Contains(agent, "chip: Apple M4\nram_gb: 24\n") {
		t.Errorf("agents keep chip and ram_gb:\n%s", agent)
	}
}

// Intent (captain): "The format human should be the default. If it's better
// for an agent, add that format agent." The test env's stdout is a pipe, so
// the bare command proves human wins even when piped; --format agent and the
// older --format toon still give TOON; other commands keep their piped TOON.
func TestCatalogDefaultsToHumanEvenWhenPiped(t *testing.T) {
	e := newEnv(t)
	bare := e.run("catalog").ok(t).out
	if !strings.Contains(bare, "ID ") || !strings.Contains(bare, "TIER 1") || strings.Contains(bare, "models[") {
		t.Errorf("a bare, piped pill catalog must print the table:\n%s", bare)
	}
	for _, f := range []string{"agent", "toon"} {
		if out := e.run("catalog", "--format", f).ok(t).out; !strings.Contains(out, "models[7]{id,") {
			t.Errorf("--format %s must give TOON:\n%s", f, out)
		}
	}
	if out := e.run("ls").ok(t).out; strings.Contains(out, "NAME") {
		t.Errorf("other commands keep TOON when piped:\n%s", out)
	}
}

// Intent (captain): the default is marked in the list, by its short ID, with
// an * to its left ("I think that is common practice"), and the IDs still
// line up under the heading.
func TestCatalogMarksTheDefault(t *testing.T) {
	e := newEnv(t)
	e.gguf("MiniCPM-V-4.7-35B-A3B-AD-IQ2_XS-IQ2_XXS.gguf")
	e.run("add", "minicpm", "--unverified").ok(t)
	e.run("default", "minicpm").ok(t)
	r := e.run("catalog", "--format", "human").ok(t)
	if !strings.Contains(r.out, "\n*minicpm✓ ") || !strings.Contains(r.out, "\n ID ") || !strings.Contains(r.out, "\n muse-30b✗ ") {
		t.Errorf("the default must carry a * to its left, aligned with the other IDs:\n%s", r.out)
	}
	if r := e.run("catalog", "--format", "agent").ok(t); !strings.Contains(r.out, "default: minicpm\n") {
		t.Errorf("agents get a default field:\n%s", r.out)
	}
}

// Intent: the champion is a catalog reference with short commands. Its short
// name pulls the exact recorded file from the recorded repository, and the
// binary only ever held the reference.
func TestChampionShortNamePullsRecordedFile(t *testing.T) {
	e := newEnv(t)
	const file = "MiniCPM-V-4.7-35B-A3B-AD-IQ2_XS-IQ2_XXS.gguf"
	hub := testutil.NewHub(t, map[string]map[string][]byte{
		"AtomicChat/MiniCPM-V-4.7-35B-A3B-GGUF": {file: []byte("pretend-champion-weights")},
	})
	r := e.run("pull", "minicpm").ok(t) // the short ID; "minicpm-v4-7" still works as an alias
	if !strings.Contains(r.out, "status: downloaded") || !strings.Contains(r.out, "file: "+file) {
		t.Errorf("pull:\n%s", r.out)
	}
	if hub.Hits("/resolve/") != 1 {
		t.Errorf("one download expected, got %d", hub.Hits("/resolve/"))
	}
	// Registering resolves the short name to the same variant and its context.
	add := e.run("add", "minicpm-v4-7", "--unverified").ok(t)
	if !strings.Contains(add.out, "model: minicpm-v4-7-adiq2xsiq2xxs-64k") {
		t.Errorf("add:\n%s", add.out)
	}
}

func TestLsEmptyThenPopulated(t *testing.T) {
	e := newEnv(t)
	r := e.run("ls").ok(t)
	golden(t, "ls_empty", r.out)

	e.gguf(gemmaFile)
	e.gguf("gemma-4-26B-A4B-it-UD-Q3_K_XL.gguf")
	if r := e.run("ls").ok(t); !strings.Contains(r.out, "gemma4-26b-iq4xs-64k,pulled,") {
		t.Errorf("unregistered files should be pulled:\n%s", r.out)
	}
	e.run("add", "gemma4-26b-iq4xs-64k", "--unverified").ok(t)
	e.run("default", "gemma4-26b-iq4xs-64k").ok(t)
	r = e.run("ls").ok(t)
	golden(t, "ls", r.out)
}

func TestLsShowsMissingDeclaredModels(t *testing.T) {
	e := newEnv(t)
	e.register()
	if err := os.Remove(filepath.Join(e.modelsDir(), gemmaFile)); err != nil {
		t.Fatal(err)
	}
	if r := e.run("ls").ok(t); !strings.Contains(r.out, "gemma4-26b-iq4xs-64k,missing,n/a,yes") {
		t.Errorf("a declared model without its file is missing:\n%s", r.out)
	}
}

func TestPullCatalogModelThenAdd(t *testing.T) {
	e := newEnv(t)
	hub := gemmaHub(t)

	r := e.run("pull", "gemma4-26b").ok(t)
	if !strings.Contains(r.out, "status: downloaded") || !strings.Contains(r.out, "file: "+gemmaFile) {
		t.Errorf("pull:\n%s", r.out)
	}
	if !strings.Contains(r.out, "help[1]: register it: pill add gemma4-26b --unverified") {
		t.Errorf("pull must point at add:\n%s", r.out)
	}
	if _, err := os.Stat(filepath.Join(e.modelsDir(), gemmaFile)); err != nil {
		t.Fatal("file missing after pull")
	}
	if _, err := os.Stat(filepath.Join(e.dir, "pill", "config", "models.toml")); err == nil {
		t.Error("pull must never register the model")
	}

	again := e.run("pull", "gemma4-26b").ok(t)
	if !strings.Contains(again.out, "status: already present") {
		t.Errorf("second pull:\n%s", again.out)
	}
	if hub.Hits("/resolve/") != 1 {
		t.Errorf("the file was downloaded %d times", hub.Hits("/resolve/"))
	}

	e.run("add", "gemma4-26b", "--unverified").ok(t)
	if r := e.run("ls").ok(t); !strings.Contains(r.out, "gemma4-26b-iq4xs-64k,unverified") {
		t.Errorf("ls after add:\n%s", r.out)
	}
}

func TestPullHuggingFaceReferenceThenAddWithContext(t *testing.T) {
	e := newEnv(t)
	gemmaHub(t)
	r := e.run("pull", "hf.co/o/r:Q4_K_M").ok(t)
	if !strings.Contains(r.out, "file: r-Q4_K_M.gguf") {
		t.Errorf("pull:\n%s", r.out)
	}
	add := e.run("add", "hf.co/o/r:Q4_K_M", "--unverified", "--ctx", "8192").ok(t)
	if !strings.Contains(add.out, "model: r-q4km-8k") || !strings.Contains(add.out, "ctx: 8192") {
		t.Errorf("add by reference:\n%s", add.out)
	}
	e.run("pull", "o/r/r-Q8_0.gguf").ok(t) // the file form
	if _, err := os.Stat(filepath.Join(e.modelsDir(), "r-Q8_0.gguf")); err != nil {
		t.Error("file-form pull missing")
	}
}

func TestPullErrors(t *testing.T) {
	e := newEnv(t)
	hub := gemmaHub(t)
	e.run("pull", "nonexistent-model").fails(t, 1, "unknown model")
	e.run("pull", "hf.co/o/r:Q9_9").fails(t, 1, "no GGUF for quantisation")
	hub.Status = 401
	r := e.run("pull", "gemma4-26b").fails(t, 1, "HTTP 401")
	if !strings.Contains(r.out, "HF_TOKEN") {
		t.Errorf("auth failures should mention HF_TOKEN:\n%s", r.out)
	}
	hub.Status = 0
	t.Setenv("HF_TOKEN", "secret-token")
	e.run("pull", "gemma4-26b").ok(t)
	if hub.LastAuth != "Bearer secret-token" {
		t.Errorf("HF_TOKEN not forwarded: %q", hub.LastAuth)
	}
	if cfg, _ := os.ReadFile(filepath.Join(e.dir, "pill", "results.json")); strings.Contains(string(cfg), "secret-token") {
		t.Error("HF_TOKEN must never be stored")
	}
}

func TestRmDeletesFileOnlyWhenUnused(t *testing.T) {
	e := newEnv(t)
	e.register()
	e.run("add", "gemma4-26b", "--unverified", "--ctx", "32768").ok(t) // second entry, same file
	file := filepath.Join(e.modelsDir(), gemmaFile)

	r := e.run("rm", "gemma4-26b-iq4xs-32k").ok(t)
	if !strings.Contains(r.out, "status: removed") || !strings.Contains(r.out, "file: kept (still used by gemma4-26b-iq4xs-64k)") {
		t.Errorf("first rm:\n%s", r.out)
	}
	if _, err := os.Stat(file); err != nil {
		t.Fatal("the shared GGUF must survive while another entry uses it")
	}

	r = e.run("rm", "gemma4-26b-iq4xs-64k").ok(t)
	if !strings.Contains(r.out, "file: deleted") || !strings.Contains(r.out, "help[1]: the default model was removed") {
		t.Errorf("second rm:\n%s", r.out)
	}
	if _, err := os.Stat(file); err == nil {
		t.Error("the GGUF should be deleted with its last entry")
	}
	if prov, _, _ := pi.ReadProvider(e.piModels()); prov != nil {
		t.Errorf("pill's provider should be gone with its last model: %s", prov)
	}
	ini, _ := os.ReadFile(filepath.Join(e.dir, "pill", "models.ini"))
	if strings.Contains(string(ini), "gemma4") {
		t.Errorf("models.ini still lists a removed model:\n%s", ini)
	}

	again := e.run("rm", "gemma4-26b-iq4xs-64k").ok(t) // idempotent
	if !strings.Contains(again.out, "status: not found") {
		t.Errorf("repeat rm:\n%s", again.out)
	}
}

func TestRmUnloadsLoadedModelAndRefreshesRouter(t *testing.T) {
	e := newEnv(t)
	e.register()
	e.run("serve").ok(t)
	e.chat("gemma4-26b-iq4xs-64k")
	e.run("rm", "gemma4-26b-iq4xs-64k").ok(t)
	if body := e.get("/v1/models"); strings.Contains(body, "gemma4") {
		t.Errorf("router still lists the removed model: %s", body)
	}
}

func TestRmPulledOnlyFile(t *testing.T) {
	e := newEnv(t)
	e.gguf(gemmaFile)
	r := e.run("rm", "gemma4-26b-iq4xs-64k").ok(t)
	if !strings.Contains(r.out, "status: removed") || !strings.Contains(r.out, "file: deleted") {
		t.Errorf("rm of a pulled file:\n%s", r.out)
	}
	if _, err := os.Stat(filepath.Join(e.modelsDir(), gemmaFile)); err == nil {
		t.Error("pulled file not deleted")
	}
}
