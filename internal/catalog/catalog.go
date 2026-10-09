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
}

// Family is one model with its variants.
type Family struct {
	Name      string         `toml:"name"`
	Repo      string         `toml:"repo"`
	Reasoning bool           `toml:"reasoning"`
	MaxTokens int            `toml:"max_tokens"`
	Preset    map[string]any `toml:"preset"`
	Variants  []Variant      `toml:"variant"`
}

// Catalog is the parsed catalog.toml.
type Catalog struct {
	Families []Family `toml:"model"`
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

// Recommended picks the variant for a machine with ramGB of memory: the
// default variant if it fits, otherwise the smallest one.
func (f *Family) Recommended(ramGB int) *Variant {
	d := f.DefaultVariant()
	if ramGB >= d.MinRAMGB {
		return d
	}
	best := &f.Variants[0]
	for i := range f.Variants {
		if f.Variants[i].SizeGB < best.SizeGB {
			best = &f.Variants[i]
		}
	}
	return best
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
