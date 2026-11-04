// Package catalog is pill's built-in list of known models.
package catalog

import (
	_ "embed" // enables the //go:embed directive below
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

// catalogTOML is compiled into the binary: `//go:embed` copies the named file
// into this string at build time, so the installed program needs no data files.
//
//go:embed catalog.toml
var catalogTOML string

// Variant is one quantisation of a model.
type Variant struct {
	Quant    string  `toml:"quant"`
	File     string  `toml:"file"`
	SizeGB   float64 `toml:"size_gb"`
	Ctx      int     `toml:"ctx"`
	MinRAMGB int     `toml:"min_ram_gb"`
	Default  bool    `toml:"default"`
	// Measurements are benchmark results for this variant, each tied to one
	// exact device profile. They are the only evidence behind a recommendation.
	Measurements []Measurement `toml:"measurement"`
}

// Measurement is one pill benchmark result on one exact device profile (chip
// and unified memory). Tier1 and Tier2 are "pass", "relaxed" (passed only
// with limits looser than pill's defaults), "fail", or empty when the tier did
// not run. A missing measurement is "no result", never a failure.
//
// The detail fields come from results.json (tier1_seconds, tier2_score,
// min_free_pct, swap_growth_mb, reason) and the Tier 3 index. A zero or nil
// field means "not measured", never a measured zero. Records backfilled from
// the reports summarise a batch by its least favourable reported value.
type Measurement struct {
	Chip     string  `toml:"chip"`
	RAMGB    int     `toml:"ram_gb"`
	Ctx      int     `toml:"ctx"`
	Tier1    string  `toml:"tier1"`
	Tier2    string  `toml:"tier2"`
	TokPerS  float64 `toml:"tok_per_s"` // 0 when not recorded
	Runs     int     `toml:"runs"`
	Date     string  `toml:"date"`
	Pill     string  `toml:"pill"`
	LlamaCpp string  `toml:"llama_cpp"`
	Gate     string  `toml:"gate"` // limits used, e.g. "tier 1 60 s, tier 2 15 min"
	Source   string  `toml:"source"`
	// SourceRow is the first cell of the report table row this record was
	// converted from, so a test can compare the two.
	SourceRow string `toml:"source_row"`
	Note      string `toml:"note"`

	Tier1Seconds  float64 `toml:"tier1_seconds"`   // time to the first reply
	Tier2Score    int     `toml:"tier2_score"`     // hidden tests passed; meaningful when Tier2Total > 0
	Tier2Total    int     `toml:"tier2_total"`     // hidden tests in the task; 0 when Tier 2 did not score
	Tier2Minutes  float64 `toml:"tier2_minutes"`   // wall time of the Tier 2 run
	Tier2TimedOut bool    `toml:"tier2_timed_out"` // the Tier 2 cap was hit
	// MinFreePct and SwapMB are pointers because zero swap is a result.
	MinFreePct *float64    `toml:"min_free_pct"` // lowest free memory during the run
	SwapMB     *float64    `toml:"swap_mb"`      // swap growth during the run
	FailReason string      `toml:"fail_reason"`  // why the run did not pass
	Tier3      []Tier3Task `toml:"tier3"`        // empty when Tier 3 did not run
}

// Tier3Task is one Tier 3 task result. Tier 3 has no pass mark; the score is
// the hidden test score out of 100. With one run, Median is that run's score.
type Tier3Task struct {
	Task   string  `toml:"task"`
	Median float64 `toml:"median"`
	Min    float64 `toml:"min"` // 0 when Runs is 1
	Max    float64 `toml:"max"` // 0 when Runs is 1
	Runs   int     `toml:"runs"`
}

// Tried is a model that was benchmarked but is not offered: it has no
// download reference, so it never appears as something to pull. It exists to
// hold its measurements, so `pill catalog` can show what a device cannot do.
type Tried struct {
	Name         string        `toml:"name"` // display name, e.g. "Qwen3-8B Q4_K_M"
	Ctx          int           `toml:"ctx"`  // 0 when not recorded
	Measurements []Measurement `toml:"measurement"`
}

// Family is one model with its variants.
type Family struct {
	Name string `toml:"name"`
	Repo string `toml:"repo"`
	// Source is where the weights come from. The catalog only ever holds
	// references (source, repo, file), never the weights. "huggingface" is the
	// only source pill can download from; Load rejects any other.
	Source    string         `toml:"source"`
	Strengths []string       `toml:"strengths"` // curated tags, from the Tier 3 reports
	Reasoning bool           `toml:"reasoning"`
	MaxTokens int            `toml:"max_tokens"`
	Preset    map[string]any `toml:"preset"`
	Variants  []Variant      `toml:"variant"`
}

// Catalog is the parsed catalog.toml.
type Catalog struct {
	Families []Family `toml:"model"`
	Tried    []Tried  `toml:"tried"`
}

// Entry is a family + variant + context size resolved from a pill name.
type Entry struct {
	Name    string // full pill name, e.g. gemma4-26b-iq4xs-64k
	Family  *Family
	Variant *Variant
	Ctx     int
}

// Load parses the embedded catalog.
func Load() (*Catalog, error) {
	var c Catalog
	if _, err := toml.Decode(catalogTOML, &c); err != nil {
		return nil, fmt.Errorf("parse embedded catalog: %w", err)
	}
	for _, f := range c.Families {
		if f.Source != "" && f.Source != "huggingface" {
			return nil, fmt.Errorf("catalog model %s: source %q is not supported (only huggingface)", f.Name, f.Source)
		}
	}
	return &c, nil
}

// ShortQuant turns "UD-IQ4_XS" into "iq4xs": drop the Unsloth "UD-" prefix,
// lower-case, strip separators. Used in model names.
func ShortQuant(quant string) string {
	q := strings.TrimPrefix(strings.ToLower(quant), "ud-")
	return strings.NewReplacer("_", "", "-", "").Replace(q)
}

// CtxLabel turns a token count into the "64k" form used in names. A count
// that is not a whole multiple of 1024 keeps its exact value ("9000tok"), so
// 8192 and 9000 never collapse into the same model name.
func CtxLabel(ctx int) string {
	if ctx%1024 == 0 {
		return strconv.Itoa(ctx/1024) + "k"
	}
	return strconv.Itoa(ctx) + "tok"
}

// NameFor builds the canonical pill name: <family>-<quant>-<ctx>.
func NameFor(family, quant string, ctx int) string {
	return family + "-" + ShortQuant(quant) + "-" + CtxLabel(ctx)
}

// ctxSuffix matches the context part of a name: "-32k" (thousands of
// 1024 tokens) or "-9000tok" (an exact count, see CtxLabel).
var ctxSuffix = regexp.MustCompile(`^(.*)-(\d+)(k|tok)$`)

// Resolve maps a user-supplied name to a catalog entry. It accepts the full
// name (gemma4-26b-iq4xs-64k), a full name with a different context
// (gemma4-26b-iq4xs-32k, same file) and the short family name (gemma4-26b,
// meaning the default variant).
func (c *Catalog) Resolve(name string) (Entry, bool) {
	for i := range c.Families {
		f := &c.Families[i]
		if name == f.Name {
			v := f.DefaultVariant()
			return Entry{Name: NameFor(f.Name, v.Quant, v.Ctx), Family: f, Variant: v, Ctx: v.Ctx}, true
		}
		for j := range f.Variants {
			v := &f.Variants[j]
			if name == NameFor(f.Name, v.Quant, v.Ctx) {
				return Entry{Name: name, Family: f, Variant: v, Ctx: v.Ctx}, true
			}
			// Same file, other context: <family>-<quant>-<N>k.
			if m := ctxSuffix.FindStringSubmatch(name); m != nil && m[1] == f.Name+"-"+ShortQuant(v.Quant) {
				n, _ := strconv.Atoi(m[2])
				if m[3] == "k" {
					n *= 1024
				}
				if n > 0 {
					return Entry{Name: name, Family: f, Variant: v, Ctx: n}, true
				}
			}
		}
	}
	return Entry{}, false
}

// ResolveWithCtx is Resolve plus an explicit context override (--ctx); the
// name is regenerated so it always reflects the context actually used.
func (c *Catalog) ResolveWithCtx(name string, ctx int) (Entry, bool) {
	e, ok := c.Resolve(name)
	if !ok || ctx == 0 || ctx == e.Ctx {
		return e, ok
	}
	e.Ctx = ctx
	e.Name = NameFor(e.Family.Name, e.Variant.Quant, ctx)
	return e, true
}

// DefaultVariant is the variant marked default, else the first.
func (f *Family) DefaultVariant() *Variant {
	for i := range f.Variants {
		if f.Variants[i].Default {
			return &f.Variants[i]
		}
	}
	return &f.Variants[0]
}

// Entries lists every catalog variant as a resolved entry.
func (c *Catalog) Entries() []Entry {
	var out []Entry
	for i := range c.Families {
		f := &c.Families[i]
		for j := range f.Variants {
			v := &f.Variants[j]
			out = append(out, Entry{Name: NameFor(f.Name, v.Quant, v.Ctx), Family: f, Variant: v, Ctx: v.Ctx})
		}
	}
	return out
}

// FindByRepoQuant finds the catalog entry for a Hugging Face repository and
// quantisation ("unsloth/...-GGUF" + "IQ4_XS"), so a reference typed as
// hf.co/owner/repo:QUANT gets the same name and settings as the catalog name.
// The quantisation is compared the way names are: case, "UD-" and separators
// are ignored.
func (c *Catalog) FindByRepoQuant(repo, quant string) (Entry, bool) {
	for _, e := range c.Entries() {
		if strings.EqualFold(e.Family.Repo, repo) && ShortQuant(e.Variant.Quant) == ShortQuant(quant) {
			return e, true
		}
	}
	return Entry{}, false
}

// FindByFile returns the entry whose GGUF file name matches.
func (c *Catalog) FindByFile(file string) (Entry, bool) {
	for _, e := range c.Entries() {
		if e.Variant.File == file {
			return e, true
		}
	}
	return Entry{}, false
}
