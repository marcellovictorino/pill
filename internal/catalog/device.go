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
	var best *Measurement
	for i := range e.Variant.Measurements {
		m := &e.Variant.Measurements[i]
		if !strings.EqualFold(m.Chip, d.Chip) || m.RAMGB != d.RAMGB || m.Ctx != e.Variant.Ctx {
			continue
		}
		if best == nil || m.Date > best.Date {
			best = m // the newest run on this device wins
		}
	}
	switch {
	case best == nil:
		return Assessment{Status: StatusUnmeasured}
	case best.Tier1 == "pass" && best.Tier2 == "pass":
		return Assessment{Status: StatusRecommended, Measurement: best}
	case best.Tier1 == "relaxed" && best.Tier2 == "pass":
		return Assessment{Status: StatusRelaxed, Measurement: best}
	}
	return Assessment{Status: StatusFailed, Measurement: best}
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
