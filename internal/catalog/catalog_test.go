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
	if len(c.Families) != 1 || c.Families[0].Name != "gemma4-26b" || len(c.Families[0].Variants) != 2 {
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

func TestRecommended(t *testing.T) {
	f := &mustLoad(t).Families[0]
	if v := f.Recommended(24); v.Quant != "UD-IQ4_XS" {
		t.Errorf("24 GB should get the default variant, got %s", v.Quant)
	}
	if v := f.Recommended(16); v.Quant != "UD-Q3_K_XL" {
		t.Errorf("16 GB should get the smallest variant, got %s", v.Quant)
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
