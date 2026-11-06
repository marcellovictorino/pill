package catalog

import "testing"

func mustLoad(t *testing.T) *Catalog {
	t.Helper()
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestEmbeddedCatalog(t *testing.T) {
	c := mustLoad(t)
	if len(c.Families) != 2 || c.Families[0].Name != "gemma4-26b" || len(c.Families[0].Variants) != 2 || c.Families[1].Name != "minicpm-v4-7" {
		t.Fatalf("unexpected catalog: %+v", c.Families)
	}
}

func TestNaming(t *testing.T) {
	if got := NameFor("gemma4-26b", "UD-IQ4_XS", 65536); got != "gemma4-26b-iq4xs-64k" {
		t.Errorf("got %s", got)
	}
	if got := NameFor("gemma4-26b", "UD-Q3_K_XL", 65536); got != "gemma4-26b-q3kxl-64k" {
		t.Errorf("got %s", got)
	}
}

// 8192 and 9000 are different context sizes and must make different names, or
// registering the second would silently replace the first.
func TestCtxLabelKeepsExactCounts(t *testing.T) {
	if CtxLabel(8192) != "8k" || CtxLabel(65536) != "64k" {
		t.Errorf("multiples of 1024 keep the short form: %s %s", CtxLabel(8192), CtxLabel(65536))
	}
	if CtxLabel(9000) == CtxLabel(8192) || CtxLabel(9000) != "9000tok" {
		t.Errorf("9000 collapsed into %s", CtxLabel(9000))
	}
	if NameFor("m", "Q4_K_M", 9000) == NameFor("m", "Q4_K_M", 8192) {
		t.Error("different contexts must produce different names")
	}
}

func TestResolve(t *testing.T) {
	c := mustLoad(t)
	cases := []struct {
		in       string
		wantName string
		wantFile string
		wantCtx  int
		ok       bool
	}{
		{"gemma4-26b", "gemma4-26b-iq4xs-64k", "gemma-4-26B-A4B-it-UD-IQ4_XS.gguf", 65536, true},
		{"gemma4-26b-iq4xs-64k", "gemma4-26b-iq4xs-64k", "gemma-4-26B-A4B-it-UD-IQ4_XS.gguf", 65536, true},
		{"gemma4-26b-q3kxl-64k", "gemma4-26b-q3kxl-64k", "gemma-4-26B-A4B-it-UD-Q3_K_XL.gguf", 65536, true},
		{"gemma4-26b-iq4xs-32k", "gemma4-26b-iq4xs-32k", "gemma-4-26B-A4B-it-UD-IQ4_XS.gguf", 32768, true},
		{"gemma4-26b-iq4xs-9000tok", "gemma4-26b-iq4xs-9000tok", "gemma-4-26B-A4B-it-UD-IQ4_XS.gguf", 9000, true}, // an exact count round-trips
		{"gemma4-26b-bogus-64k", "", "", 0, false},
		{"other", "", "", 0, false},
	}
	for _, tc := range cases {
		e, ok := c.Resolve(tc.in)
		if ok != tc.ok {
			t.Errorf("%s: ok=%v, want %v", tc.in, ok, tc.ok)
			continue
		}
		if ok && (e.Name != tc.wantName || e.Variant.File != tc.wantFile || e.Ctx != tc.wantCtx) {
			t.Errorf("%s: got %+v", tc.in, e)
		}
	}
}

func TestResolveWithCtxRenames(t *testing.T) {
	c := mustLoad(t)
	e, ok := c.ResolveWithCtx("gemma4-26b", 32768)
	if !ok || e.Name != "gemma4-26b-iq4xs-32k" || e.Ctx != 32768 {
		t.Errorf("got %+v", e)
	}
}

func TestFindByFile(t *testing.T) {
	c := mustLoad(t)
	if e, ok := c.FindByFile("gemma-4-26B-A4B-it-UD-Q3_K_XL.gguf"); !ok || e.Name != "gemma4-26b-q3kxl-64k" {
		t.Errorf("got %+v %v", e, ok)
	}
	if _, ok := c.FindByFile("nope.gguf"); ok {
		t.Error("unexpected match")
	}
}

func TestShortQuantIgnoresCaseOfThePrefix(t *testing.T) {
	for _, q := range []string{"UD-IQ4_XS", "ud-iq4_xs", "IQ4_XS", "iq4_xs"} {
		if got := ShortQuant(q); got != "iq4xs" {
			t.Errorf("ShortQuant(%q) = %q", q, got)
		}
	}
	c := mustLoad(t)
	e, ok := c.FindByRepoQuant("unsloth/gemma-4-26B-A4B-it-GGUF", "ud-iq4_xs")
	if !ok || e.Ctx != 65536 {
		t.Errorf("a lowercase reference must find the catalog entry: %+v %v", e, ok)
	}
}

// Intent (captain): short readable IDs, with the real source kept in the
// catalog file. Every model needs an ID, and one ID + quant + context must name
// exactly one record, or two rows of `pill catalog` would be indistinguishable.
func TestEveryModelHasAShortIDAndTheKeyIsUnique(t *testing.T) {
	c := mustLoad(t)
	seen := map[string]string{}
	add := func(id, quant string, ctx int, what string) {
		key := id + "|" + ShortQuant(quant) + "|" + CtxLabel(ctx)
		if id == "" || quant == "" {
			t.Errorf("%s has no id or quant", what)
		}
		// The widest human table line (the heading, beside a 23 character
		// Tier 3 cell and a 7 character Tier 1 word) is 88 characters with a
		// 17 character ID.
		if len(id) > MaxIDLen {
			t.Errorf("%s: id %q is longer than %d characters and would wrap the list", what, id, MaxIDLen)
		}
		if prev, dup := seen[key]; dup {
			t.Errorf("%s and %s share the key %s", what, prev, key)
		}
		seen[key] = what
	}
	for _, f := range c.Families {
		for _, v := range f.Variants {
			add(f.PublicID(), v.Quant, v.Ctx, f.Name+" "+v.Quant)
		}
	}
	for _, tr := range c.Tried {
		add(tr.ID, tr.Quant, tr.Ctx, tr.Name)
	}
	if e, ok := c.Resolve("minicpm"); !ok || e.Family.Repo == "" || e.Variant.File == "" {
		t.Errorf("the ID minicpm must map to a repository and file: %+v", e)
	}
}

// Intent (captain): the Tier 3 column has a fixed task order, so every task a
// record names must have a place in it.
func TestEveryTier3TaskHasAPlaceInTheColumn(t *testing.T) {
	known := map[string]bool{}
	for _, g := range Tier3Order {
		for _, task := range g {
			known[task] = true
		}
	}
	c := mustLoad(t)
	check := func(ms []Measurement) {
		for _, m := range ms {
			for _, t3 := range m.Tier3 {
				if !known[t3.Task] {
					t.Errorf("Tier 3 task %q is not in Tier3Order", t3.Task)
				}
			}
		}
	}
	for _, f := range c.Families {
		for _, v := range f.Variants {
			check(v.Measurements)
		}
	}
	for _, tr := range c.Tried {
		check(tr.Measurements)
	}
}

// Intent (captain): the QUANT column is the bit width; the exact quant only
// where two files of one model share a bit width (Gemma 4 26B has two Q3 files).
func TestQuantLabel(t *testing.T) {
	c := mustLoad(t)
	for _, tc := range []struct{ id, quant, want string }{
		{"minicpm", "AD-IQ2_XS-IQ2_XXS", "Q2"},
		{"gemma4-26b", "UD-IQ4_XS", "Q4"},
		{"gemma4-26b", "UD-IQ3_S", "IQ3S"},
		{"gemma4-26b", "UD-Q3_K_XL", "Q3KXL"},
		{"lfm2.5-24b-a2b", "Q4_K_M", "Q4"},
		{"lfm2.5-24b-a2b", "UD-Q3_K_XL", "Q3"},
		{"qwen3.5-9b", "UD-Q4_K_XL", "Q4"},
	} {
		if got := c.QuantLabel(tc.id, tc.quant); got != tc.want {
			t.Errorf("%s %s: %s, want %s", tc.id, tc.quant, got, tc.want)
		}
	}
}

// Intent (captain): commands take the short ID, id:quant when an ID has two
// files, and the old long names keep working as aliases.
func TestResolveShortIDs(t *testing.T) {
	c := mustLoad(t)
	for _, tc := range []struct{ in, wantName string }{
		{"minicpm", "minicpm-v4-7-adiq2xsiq2xxs-64k"},
		{"minicpm:ad-iq2_xs-iq2_xxs", "minicpm-v4-7-adiq2xsiq2xxs-64k"},
		{"minicpm-v4-7", "minicpm-v4-7-adiq2xsiq2xxs-64k"},
		{"gemma4-26b", "gemma4-26b-iq4xs-64k"},
		{"gemma4-26b:q3kxl", "gemma4-26b-q3kxl-64k"},
		{"gemma4-26b:UD-Q3_K_XL", "gemma4-26b-q3kxl-64k"},
	} {
		if e, ok := c.Resolve(tc.in); !ok || e.Name != tc.wantName {
			t.Errorf("%s: %+v %v, want %s", tc.in, e, ok, tc.wantName)
		}
	}
	for _, bad := range []string{"gemma4-26b:iq3s", "minicpm:q8", "nope:q4"} {
		if _, ok := c.Resolve(bad); ok {
			t.Errorf("%s must not resolve (tried-only or unknown quant)", bad)
		}
	}
}
