package catalog

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Tier3Order is the fixed order of Tier 3 scores in `pill catalog`. Scores that
// belong to one exercise (three landing-page briefs) form a group, so the
// column reads Go|cleaning|insight|Kanban|(nursery,Mars,tax).
var Tier3Order = [][]string{
	{"go-notes"},
	{"py-clean"},
	{"py-insight"},
	{"web-kanban"},
	{"web-landing-nursery", "web-landing-mars", "web-landing-tax"},
}

// Row is one model on one device: what `pill catalog` prints for it. Offered
// rows (a catalog download) carry their Entry; tried rows only hold results.
type Row struct {
	ID          string
	Quant       string // bit width ("Q4"), or the exact quant when two files of the ID share one
	Ctx         int    // 0 when not recorded
	Status      string // yes, relaxed, failed, or unmeasured
	Measurement *Measurement
	Entry       *Entry // nil for a tried model
	Source      string // Hugging Face reference; empty for a tried model
}

// MaxIDLen is the longest model ID that keeps a `pill catalog` row under 90
// characters beside the widest Tier 3 and Tier 1 cells.
const MaxIDLen = 17

var firstDigits = regexp.MustCompile(`\d+`)

// QuantBits is the bit width in a quant name: 4 for "UD-IQ4_XS", 2 for
// "AD-IQ2_XS-IQ2_XXS" (the first number), 0 when there is none.
func QuantBits(quant string) int {
	n, _ := strconv.Atoi(firstDigits.FindString(quant))
	return n
}

// QuantLabel is the QUANT column: the bit width ("Q4"). Only when two files of
// one model ID share a bit width (Gemma 4 26B has IQ3_S and Q3_K_XL) does it
// show the exact quant ("IQ3S", "Q3KXL"), because the bit width no longer
// tells them apart. Commands then take the quant after a colon: gemma4-26b:q3kxl.
func (c *Catalog) QuantLabel(id, quant string) string {
	bits := QuantBits(quant)
	exact := strings.ToUpper(ShortQuant(quant))
	if bits == 0 {
		return exact
	}
	for _, q := range c.quantsOf(id) {
		if ShortQuant(q) != ShortQuant(quant) && QuantBits(q) == bits {
			return exact
		}
	}
	return "Q" + strconv.Itoa(bits)
}

// quantsOf lists every quant of one model ID, offered or only tried.
func (c *Catalog) quantsOf(id string) []string {
	var out []string
	for i := range c.Families {
		f := &c.Families[i]
		if f.PublicID() == id {
			for _, v := range f.Variants {
				out = append(out, v.Quant)
			}
		}
	}
	for _, t := range c.Tried {
		if t.ID == id {
			out = append(out, t.Quant)
		}
	}
	return out
}

// Rows is the `pill catalog` list for a device, one row per model. Passes come
// first (fastest first, at most MaxRecommendations), then the closest failed or
// relaxed runs (at most MaxAttempts, plus every offered model that was run,
// since a failed default must stay visible), then offered models that fit but
// were never measured here. hidden counts measured rows left out, and tooSmall
// counts offered models that need more memory and have no result on this
// device. An unknown device lists every offered model as unmeasured.
func (c *Catalog) Rows(d Device) (rows []Row, hidden, tooSmall int) {
	if !d.Known() {
		for _, e := range c.Entries() {
			rows = append(rows, c.offeredRow(e, nil, StatusUnverified))
		}
		return rows, 0, 0
	}
	var passes, near, unmeasured []Row
	for _, r := range c.Shortlist(d) {
		passes = append(passes, c.offeredRow(r.Entry, r.Measurement, StatusRecommended))
	}
	for _, e := range c.Entries() {
		m := newestOn(d, e.Variant.Measurements, e.Variant.Ctx)
		switch {
		case m != nil && verdict(m) == StatusRecommended:
			// A pass: Shortlist already holds it, or cut it at the limit.
		case m != nil:
			near = append(near, c.offeredRow(e, m, verdict(m)))
		case d.RAMGB < e.Variant.MinRAMGB:
			tooSmall++
		default:
			unmeasured = append(unmeasured, c.offeredRow(e, nil, StatusUnmeasured))
		}
	}
	for i := range c.Tried {
		t := &c.Tried[i]
		if m := newestOn(d, t.Measurements, 0); m != nil {
			near = append(near, Row{ID: t.ID, Quant: c.QuantLabel(t.ID, t.Quant), Ctx: m.Ctx, Status: verdict(m), Measurement: m})
		}
	}
	sort.SliceStable(near, func(i, j int) bool { return closer(near[i].Measurement, near[j].Measurement) })
	var shown []Row
	for i, r := range near {
		if i < MaxAttempts || r.Entry != nil {
			shown = append(shown, r)
		} else {
			hidden++
		}
	}
	rows = append(append(passes, shown...), unmeasured...)
	return rows, hidden, tooSmall
}

func (c *Catalog) offeredRow(e Entry, m *Measurement, status string) Row {
	id := e.Family.PublicID()
	e2 := e
	return Row{
		ID:          id,
		Quant:       c.QuantLabel(id, e.Variant.Quant),
		Ctx:         e.Variant.Ctx,
		Status:      status,
		Measurement: m,
		Entry:       &e2,
		Source:      "hf.co/" + e.Family.Repo + ":" + e.Variant.Quant,
	}
}

// closer orders failed or relaxed runs, closest to a pass first: more Tier 2
// tests passed, then a better Tier 1 result, then faster decoding.
func closer(a, b *Measurement) bool {
	if sa, sb := a.tier2Fraction(), b.tier2Fraction(); sa != sb {
		return sa > sb
	}
	if ra, rb := tier1Rank(a.Tier1), tier1Rank(b.Tier1); ra != rb {
		return ra > rb
	}
	return a.TokPerS > b.TokPerS
}
