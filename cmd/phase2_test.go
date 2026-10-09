package cmd

import (
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

func TestCatalogRecommendsForRAM(t *testing.T) {
	e := newEnv(t)
	r := e.run("catalog").ok(t)
	golden(t, "catalog", r.out)

	e.sys.RAM = 16 << 30 // a 16 GB machine gets the smaller variant
	r = e.run("catalog").ok(t)
	if !strings.Contains(r.out, "gemma4-26b-q3kxl-64k,12.9 GB,65536,yes,no") {
		t.Errorf("16 GB should recommend Q3_K_XL:\n%s", r.out)
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
