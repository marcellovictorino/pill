package catalog

import (
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
)

// fixture builds a catalog whose measurements are all on "Apple M9" with 32 GB,
// so each test controls exactly which evidence exists.
func fixture(t *testing.T) *Catalog {
	t.Helper()
	const data = `
[[model]]
name = "fast"
source = "huggingface"
repo = "o/fast"
strengths = ["pages"]
[[model.variant]]
quant = "Q4"
file = "fast.gguf"
size_gb = 8
ctx = 32768
min_ram_gb = 16
default = true
[[model.variant.measurement]]
chip = "Apple M9"
ram_gb = 32
ctx = 32768
tier1 = "pass"
tier2 = "pass"
tok_per_s = 40
date = "2026-01-01"
[[model.variant.measurement]]
chip = "Apple M9"
ram_gb = 32
ctx = 32768
tier1 = "fail"
date = "2026-02-01"

[[model]]
name = "slow"
repo = "o/slow"
[[model.variant]]
quant = "Q4"
file = "slow.gguf"
size_gb = 8
ctx = 32768
min_ram_gb = 16
default = true
[[model.variant.measurement]]
chip = "Apple M9"
ram_gb = 32
ctx = 32768
tier1 = "relaxed"
tier2 = "pass"
tok_per_s = 90
date = "2026-01-01"

[[model]]
name = "steady"
repo = "o/steady"
[[model.variant]]
quant = "Q4"
file = "steady.gguf"
size_gb = 8
ctx = 32768
min_ram_gb = 16
default = true
[[model.variant.measurement]]
chip = "Apple M9"
ram_gb = 32
ctx = 32768
tier1 = "pass"
tier2 = "pass"
tok_per_s = 20
date = "2026-01-01"

[[model]]
name = "huge"
repo = "o/huge"
[[model.variant]]
quant = "Q4"
file = "huge.gguf"
size_gb = 40
ctx = 32768
min_ram_gb = 64
default = true
`
	var c Catalog
	if _, err := toml.Decode(data, &c); err != nil {
		t.Fatal(err)
	}
	return &c
}

func status(c *Catalog, d Device, name string) string {
	e, _ := c.Resolve(name)
	return c.Assess(d, e).Status
}

// Intent: the shortlist borrows nothing from another device. The same chip
// with different memory, or another chip with the same memory, has no results.
func TestAssessMatchesExactProfile(t *testing.T) {
	c := fixture(t)
	if got := status(c, Device{"Apple M9", 32}, "steady"); got != StatusRecommended {
		t.Errorf("exact profile: %s", got)
	}
	for _, d := range []Device{{"Apple M9", 24}, {"Apple M9 Pro", 32}, {"Apple M8", 64}} {
		if got := status(c, d, "steady"); got == StatusRecommended || got == StatusFailed {
			t.Errorf("%+v borrowed another device's result: %s", d, got)
		}
	}
	if got := status(c, Device{"Apple M9", 24}, "steady"); got != StatusUnmeasured {
		t.Errorf("fits but unmeasured should say so, got %s", got)
	}
}

// Intent: a failed run is a result, a relaxed pass is never a standard pass,
// and the newest measurement on the device wins.
func TestAssessFailureRelaxedAndNewest(t *testing.T) {
	c := fixture(t)
	d := Device{"apple m9", 32} // chip names compare case-insensitively
	if got := status(c, d, "fast"); got != StatusFailed {
		t.Errorf("newest run failed, got %s", got)
	}
	if got := status(c, d, "slow"); got != StatusRelaxed {
		t.Errorf("relaxed limits are not a pass, got %s", got)
	}
}

// Intent: never recommend a model below its stated minimum memory, and say so
// instead of guessing when detection failed.
func TestAssessTooSmallAndUnverified(t *testing.T) {
	c := fixture(t)
	if got := status(c, Device{"Apple M9", 32}, "huge"); got != StatusTooSmall {
		t.Errorf("got %s", got)
	}
	for _, d := range []Device{{}, {Chip: "Apple M9"}, {RAMGB: 32}} {
		if got := status(c, d, "steady"); got != StatusUnverified {
			t.Errorf("%+v should be unverified, got %s", d, got)
		}
	}
	if len(c.Shortlist(Device{})) != 0 {
		t.Error("unknown device must have an empty shortlist")
	}
}

// Intent: show up to three, standard passes only, fastest first; relaxed and
// failed results never appear, and fewer (or none) is a valid answer.
func TestShortlistOnlyStandardPassesFastestFirst(t *testing.T) {
	c := fixture(t)
	got := c.Shortlist(Device{"Apple M9", 32})
	if len(got) != 1 || got[0].Entry.Family.Name != "steady" {
		t.Fatalf("only steady passed both standard tiers: %+v", got)
	}
	if n := len(c.Shortlist(Device{"Apple M9", 16})); n != 0 {
		t.Errorf("no evidence on 16 GB, got %d recommendations", n)
	}
	// Add enough passing variants to exceed the cap and check order.
	for i, tps := range []float64{50, 30, 10} {
		f := Family{Name: "extra" + string(rune('a'+i)), Variants: []Variant{{Quant: "Q", File: "x", Ctx: 32768, MinRAMGB: 8,
			Measurements: []Measurement{{Chip: "Apple M9", RAMGB: 32, Ctx: 32768, Tier1: "pass", Tier2: "pass", TokPerS: tps}}}}}
		c.Families = append(c.Families, f)
	}
	got = c.Shortlist(Device{"Apple M9", 32})
	if len(got) != MaxRecommendations || got[0].Measurement.TokPerS != 50 || got[1].Measurement.TokPerS != 30 || got[2].Measurement.TokPerS != 20 {
		t.Errorf("want top three by speed 50, 30, 20: %+v", got)
	}
}

// Intent (captain): the catalog never holds model weights, only references,
// and every measurement is attributable to a device and a source.
func TestEmbeddedCatalogHoldsReferencesAndAttributableMeasurements(t *testing.T) {
	c := mustLoad(t)
	if len(catalogTOML) > 64<<10 {
		t.Errorf("embedded catalog is %d bytes: it must hold references, not weights", len(catalogTOML))
	}
	for _, e := range c.Entries() {
		if e.Family.Repo == "" || e.Variant.File == "" || !strings.HasSuffix(e.Variant.File, ".gguf") {
			t.Errorf("%s lacks a repo/file reference", e.Name)
		}
		for _, m := range e.Variant.Measurements {
			if m.Chip == "" || m.RAMGB == 0 || m.Ctx == 0 || m.Date == "" || m.Pill == "" || m.LlamaCpp == "" || m.Source == "" {
				t.Errorf("%s: measurement is not attributable: %+v", e.Name, m)
			}
		}
	}
}

// Intent: the recorded champion is a recommendation on the tested Mac, and the
// recorded 16 GB failures never turn into one on a 16 GB Mac.
func TestChampionOnTestedMacOnly(t *testing.T) {
	c := mustLoad(t)
	got := c.Shortlist(Device{"Apple M4", 24})
	if len(got) != 1 || got[0].Entry.Name != "minicpm-v4-7-adiq2xsiq2xxs-64k" {
		t.Fatalf("champion should be the one recommendation on an M4 with 24 GB: %+v", got)
	}
	if n := len(c.Shortlist(Device{"Apple M2 Pro", 16})); n != 0 {
		t.Errorf("a 16 GB M2 Pro has no standard pass, got %d", n)
	}
	if e, ok := c.Resolve("minicpm-v4-7"); !ok || e.Variant.File != "MiniCPM-V-4.7-35B-A3B-AD-IQ2_XS-IQ2_XXS.gguf" {
		t.Errorf("short name must resolve to the champion file: %+v", e)
	}
}
