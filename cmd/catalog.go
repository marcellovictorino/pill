package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/marcellovictorino/pill/internal/catalog"
	"github.com/marcellovictorino/pill/internal/output"
	"github.com/marcellovictorino/pill/internal/sysinfo"
)

func newCatalogCmd(a *App) *cobra.Command {
	return &cobra.Command{
		Use:   "catalog",
		Short: "List the built-in models and the ones measured to work on this exact Mac",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := a.load(); err != nil {
				return err
			}
			// Detection failures are not fatal: an unknown chip or memory gives
			// an explicit "unverified" state and no recommendation.
			dev := catalog.Device{}
			if b, err := a.deps.Sys.TotalRAMBytes(); err == nil {
				dev.RAMGB = sysinfo.RAMGB(b)
			}
			if chip, err := a.deps.Sys.ChipName(); err == nil {
				dev.Chip = chip
			}
			cat := a.reg.Catalog
			var table []output.Obj
			fits := false
			for _, e := range cat.Entries() {
				local := "no"
				if _, err := os.Stat(filepath.Join(a.settings.ModelsDir, e.Variant.File)); err == nil {
					local = "yes"
				}
				if st := cat.Assess(dev, e).Status; st != catalog.StatusTooSmall {
					fits = true
				}
				table = append(table, output.Obj{}.
					Set("name", e.Name).
					Set("size", fmt.Sprintf("%.1f GB", e.Variant.SizeGB)).
					Set("ctx", e.Ctx).
					Set("recommended", cat.Assess(dev, e).Status).
					Set("local", local))
			}
			var recs []output.Obj
			for _, r := range cat.Shortlist(dev) {
				m := r.Measurement
				recs = append(recs, output.Obj{}.
					Set("name", r.Entry.Name).
					Set("strengths", strings.Join(r.Entry.Family.Strengths, ", ")).
					Set("tier1", m.Tier1).
					Set("tier2", m.Tier2).
					Set("tok_per_s", m.TokPerS).
					Set("memory", memoryLabel(m)).
					Set("tier3", tier3Label(m)).
					Set("measured", measuredLabel(m)).
					Set("note", m.Note))
			}
			// Fewer than three passes: show what else was tried on this device,
			// closest to a pass first, so the best available is still visible.
			var tried []output.Obj
			var triedMore int
			if len(recs) < catalog.MaxRecommendations {
				attempts := cat.Attempts(dev)
				if len(attempts) > catalog.MaxAttempts {
					// The closest MaxAttempts, plus every offered model: a failed
					// default must be visible even when it ranks low.
					var shown []catalog.Attempt
					for i, at := range attempts {
						if i < catalog.MaxAttempts || at.Offered {
							shown = append(shown, at)
						}
					}
					triedMore = len(attempts) - len(shown)
					attempts = shown
				}
				for _, at := range attempts {
					tried = append(tried, attemptRow(at))
				}
			}
			help := []string{"download one: pill pull <name>", "any Hugging Face GGUF works too: pill pull hf.co/owner/repo:Q4_K_M"}
			switch {
			case !dev.Known():
				help = append([]string{"could not detect this Mac's chip or memory, so nothing is recommended; pill pull <name> still works"}, help...)
			case !fits:
				help = append([]string{"no catalog model fits this Mac's memory; try a smaller GGUF from Hugging Face and test it with pill bench run"}, help...)
			case len(recs) == 0:
				help = append([]string{"no catalog model has passed both standard benchmark tiers on this exact chip and memory yet; unmeasured rows fit but are untested here (pill bench run <name> tests one)"}, help...)
			}
			chip := dev.Chip
			if chip == "" {
				chip = "unknown"
			}
			if triedMore > 0 {
				help = append(help, fmt.Sprintf("%d more tried results are in the catalog records; the closest %d and every offered model are shown", triedMore, catalog.MaxAttempts))
			}
			out := output.Obj{}.
				Set("chip", chip).
				Set("ram_gb", dev.RAMGB).
				Set("recommendations", recs)
			if len(recs) < catalog.MaxRecommendations && dev.Known() {
				out = out.Set("tried_on_this_mac", tried)
			}
			a.printer.Emit(out.
				Set("catalog", table).
				Set("help", help))
			return nil
		},
	}
}

// attemptRow is one line of the "tried on this Mac" block. Values that were
// not measured print as "-", never as zero.
func attemptRow(at catalog.Attempt) output.Obj {
	m := at.Measurement
	t1 := m.Tier1
	if t1 == "" {
		t1 = "-"
	}
	if m.Tier1Seconds > 0 {
		t1 += fmt.Sprintf(" %gs", m.Tier1Seconds)
	}
	t2 := "-"
	if m.Tier2Total > 0 {
		t2 = fmt.Sprintf("%d/%d", m.Tier2Score, m.Tier2Total)
		if m.Tier2Minutes > 0 {
			t2 += fmt.Sprintf(" in %g min", m.Tier2Minutes)
		}
		if m.Tier2TimedOut {
			t2 += ", cap hit"
		}
	}
	var tok any = "-"
	if m.TokPerS > 0 {
		tok = m.TokPerS
	}
	reason := m.FailReason
	if reason == "" {
		reason = "-"
	}
	return output.Obj{}.
		Set("name", at.Name).
		Set("verdict", at.Status).
		Set("tier1", t1).
		Set("tier2", t2).
		Set("tok_per_s", tok).
		Set("memory", memoryLabel(m)).
		Set("tier3", tier3Label(m)).
		Set("reason", reason)
}

// memoryLabel is the lowest free memory and swap growth of a run, or "-" when
// neither was recorded. Zero swap is a result, so it prints as "+0 MB".
func memoryLabel(m *catalog.Measurement) string {
	var parts []string
	if m.MinFreePct != nil {
		parts = append(parts, fmt.Sprintf("%g%% free", *m.MinFreePct))
	}
	if m.SwapMB != nil {
		parts = append(parts, fmt.Sprintf("swap +%g MB", *m.SwapMB))
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, ", ")
}

// tier3Label lists the Tier 3 score of each task ("-" when Tier 3 did not run).
// Tier 3 has no pass mark, so these are informational scores out of 100.
func tier3Label(m *catalog.Measurement) string {
	if len(m.Tier3) == 0 {
		return "-"
	}
	var parts []string
	for _, t := range m.Tier3 {
		parts = append(parts, fmt.Sprintf("%s %g", t.Task, t.Median))
	}
	return strings.Join(parts, ", ")
}

// measuredLabel is "date, pill version, N runs", leaving out what the record
// does not hold (backfilled records carry no pill version).
func measuredLabel(m *catalog.Measurement) string {
	parts := []string{m.Date}
	if m.Pill != "" {
		parts = append(parts, "pill "+m.Pill)
	}
	return strings.Join(append(parts, fmt.Sprintf("%d runs", m.Runs)), ", ")
}
