package catalog

import (
	"sort"
	"strings"
)

// Device is the machine pill runs on: exact chip name and unified memory. A
// zero Device (or one with no chip or no memory) means detection failed.
type Device struct {
	Chip  string
	RAMGB int
}

// Known reports whether both the chip and the memory were detected.
func (d Device) Known() bool { return d.Chip != "" && d.RAMGB > 0 }

// Status of one catalog variant on a device.
const (
	StatusRecommended = "yes"        // passed both standard tiers on this exact device
	StatusUnmeasured  = "unmeasured" // fits, but nobody has measured it on this device
	StatusRelaxed     = "relaxed"    // passed only with looser limits than pill's defaults
	StatusFailed      = "failed"     // measured on this device and did not pass
	StatusTooSmall    = "too-small"  // needs more memory than this device has
	StatusUnverified  = "unverified" // chip or memory could not be detected
)

// Assessment is the verdict for one entry plus the measurement behind it.
type Assessment struct {
	Status      string
	Measurement *Measurement // nil unless Status is yes, relaxed or failed
}

// Assess judges one variant on a device. Matching is exact: another chip, or
// the same chip with a different memory size, is a different device, so its
// results are never borrowed.
func (c *Catalog) Assess(d Device, e Entry) Assessment {
	if !d.Known() {
		return Assessment{Status: StatusUnverified}
	}
	if d.RAMGB < e.Variant.MinRAMGB {
		return Assessment{Status: StatusTooSmall}
	}
	best := newestOn(d, e.Variant.Measurements, e.Variant.Ctx)
	if best == nil {
		return Assessment{Status: StatusUnmeasured}
	}
	return Assessment{Status: verdict(best), Measurement: best}
}

// newestOn picks the newest measurement on exactly this device. ctx 0 matches
// any context (tried models, which record no context to match).
func newestOn(d Device, ms []Measurement, ctx int) *Measurement {
	var best *Measurement
	for i := range ms {
		m := &ms[i]
		if !strings.EqualFold(m.Chip, d.Chip) || m.RAMGB != d.RAMGB || (ctx != 0 && m.Ctx != ctx) {
			continue
		}
		if best == nil || m.Date > best.Date {
			best = m // the newest run on this device wins
		}
	}
	return best
}

// MaxSwapMB is the swap growth beyond which a run does not count as a pass:
// a model that pushes more than 4 GB to swap is not usable on that Mac, even
// when its tests pass.
const MaxSwapMB = 4096

func verdict(m *Measurement) string {
	switch {
	case m.SwapMB != nil && *m.SwapMB > MaxSwapMB:
		return StatusFailed
	case m.Tier1 == "pass" && m.Tier2 == "pass":
		return StatusRecommended
	case m.Tier1 == "relaxed" && m.Tier2 == "pass":
		return StatusRelaxed
	}
	return StatusFailed
}

// Attempt is a model that was run on this device and did not pass the
// standard tiers: a failed or relaxed result, with the numbers behind it.
type Attempt struct {
	Name        string
	Ctx         int
	Status      string // StatusFailed or StatusRelaxed
	Measurement *Measurement
}

// MaxAttempts is how many attempts the "tried" block shows.
const MaxAttempts = 5

// Attempts lists failed and relaxed runs on exactly this device, closest to a
// pass first: more Tier 2 tests passed, then a better Tier 1 result, then
// faster decoding. It looks at catalog variants (even ones too large to be
// offered, since a measured failure is a result) and tried models. A model
// measured only on other devices is absent.
func (c *Catalog) Attempts(d Device) []Attempt {
	if !d.Known() {
		return nil
	}
	var out []Attempt
	add := func(name string, ctx int, ms []Measurement) {
		if m := newestOn(d, ms, ctx); m != nil {
			if st := verdict(m); st != StatusRecommended {
				out = append(out, Attempt{Name: name, Ctx: ctx, Status: st, Measurement: m})
			}
		}
	}
	for _, e := range c.Entries() {
		add(e.Name, e.Variant.Ctx, e.Variant.Measurements)
	}
	for _, t := range c.Tried {
		add(t.Name, 0, t.Measurements)
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i].Measurement, out[j].Measurement
		if sa, sb := a.tier2Fraction(), b.tier2Fraction(); sa != sb {
			return sa > sb
		}
		if ra, rb := tier1Rank(a.Tier1), tier1Rank(b.Tier1); ra != rb {
			return ra > rb
		}
		return a.TokPerS > b.TokPerS
	})
	return out
}

// tier2Fraction is the share of Tier 2 tests passed; 0 when it did not score.
func (m *Measurement) tier2Fraction() float64 {
	if m.Tier2Total == 0 {
		return 0
	}
	return float64(m.Tier2Score) / float64(m.Tier2Total)
}

func tier1Rank(s string) int {
	switch s {
	case "pass":
		return 2
	case "relaxed":
		return 1
	}
	return 0
}

// Recommendation is one shortlisted entry.
type Recommendation struct {
	Entry       Entry
	Measurement *Measurement
}

// MaxRecommendations is how many models the shortlist shows. Fewer (even none)
// are shown when the evidence does not support more.
const MaxRecommendations = 3

// Shortlist returns up to three entries that passed both standard tiers on
// exactly this device, fastest first.
func (c *Catalog) Shortlist(d Device) []Recommendation {
	var out []Recommendation
	for _, e := range c.Entries() {
		if a := c.Assess(d, e); a.Status == StatusRecommended {
			out = append(out, Recommendation{Entry: e, Measurement: a.Measurement})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Measurement.TokPerS > out[j].Measurement.TokPerS })
	if len(out) > MaxRecommendations {
		out = out[:MaxRecommendations]
	}
	return out
}
