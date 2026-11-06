package cmd

import (
	"fmt"
	"math"
	"strconv"
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
			rows, hidden, tooSmall := a.reg.Catalog.Rows(dev)
			var models []output.Obj
			defaultID := ""
			for _, r := range rows {
				isDefault := r.Entry != nil && r.Entry.Name == a.settings.Default
				if isDefault {
					defaultID = r.ID
				}
				models = append(models, catalogRow(r, a.printer, isDefault))
			}
			var help []string
			switch {
			case !dev.Known():
				help = append(help, "could not detect this Mac's chip or memory, so nothing is measured here; pill pull <id> still works")
			case len(rows) == 0:
				help = append(help, "no catalog model fits this Mac's memory; try a smaller GGUF from Hugging Face and test it with pill bench run")
			case !hasStatus(rows, catalog.StatusRecommended):
				help = append(help, "no model has passed both standard benchmark tiers on this exact chip and memory yet")
			}
			help = append(help,
				"test one on this Mac: pill bench run <id> (a Q3KXL style quant goes after a colon: <id>:q3kxl)",
				"make one your default: pill default <id>")
			if hidden > 0 {
				help = append(help, fmt.Sprintf("%d more tried results are in the catalog records", hidden))
			}
			if tooSmall > 0 {
				help = append(help, fmt.Sprintf("catalog models that need more memory than this Mac has: %d", tooSmall))
			}
			chip := dev.Chip
			if chip == "" {
				chip = "unknown"
			}
			if a.printer.Human {
				// People read a report: plain labels and units, then the table.
				memory := "unknown"
				if dev.RAMGB > 0 {
					memory = fmt.Sprintf("%dGB RAM", dev.RAMGB)
				}
				a.printer.Report(output.Obj{}.Set("Chip", chip).Set("Memory", memory), models, help)
				return nil
			}
			out := output.Obj{}.Set("chip", chip).Set("ram_gb", dev.RAMGB)
			if defaultID != "" {
				out = out.Set("default", defaultID) // agents get the default as a field; people see the * in the table
			}
			a.printer.Emit(out.Set("models", models).Set("help", help))
			return nil
		},
	}
}

func hasStatus(rows []catalog.Row, status string) bool {
	for _, r := range rows {
		if r.Status == status {
			return true
		}
	}
	return false
}

// catalogRow is one model: ID, QUANT, CTX, then the results as short as they
// can be read (README: "Catalog columns"). Values that were not measured print
// as "-", never as zero. Agents also get the Hugging Face source, which would
// only wrap a terminal table.
func catalogRow(r catalog.Row, p *output.Printer, isDefault bool) output.Obj {
	human := p.Human
	m := r.Measurement
	id := r.ID
	row := output.Obj{}
	if human {
		// A gutter column (no key, so no heading and no gap) holds "*" for the
		// default, just left of its ID. The verdict mark is glued to the ID: a
		// column of its own would push rows past 90 characters.
		mark := " "
		if isDefault {
			mark = p.Paint("1;33", "*")
		}
		row = row.Set("", mark)
		id += verdictMark(r, p)
	}
	ctx := "-"
	if r.Ctx > 0 {
		ctx = catalog.CtxLabel(r.Ctx)
	}
	t1, t2, free, t3 := "-", "-", "-", "-"
	var tok any = "-"
	if m != nil {
		if m.Tier1 != "" {
			t1 = m.Tier1
			if t1 == "relaxed" {
				t1 = "fail" // passed only with looser limits: its time is over pill's cutoff
			}
			if m.Tier1Seconds > 0 {
				t1 += fmt.Sprintf(" (%.0fs)", m.Tier1Seconds) // "pass (48s)"
			}
		}
		if m.Tier2Total > 0 {
			t2 = fmt.Sprintf("%d/%d", m.Tier2Score, m.Tier2Total)
			if m.Tier2Minutes > 0 {
				t2 += " " + minutesLabel(m.Tier2Minutes) // "22/22 4.9m"
			}
		}
		if m.TokPerS > 0 {
			tok = m.TokPerS
		}
		if m.MinFreePct != nil {
			free = fmt.Sprintf("%g%%", *m.MinFreePct)
		}
		t3 = tier3Label(m)
	}
	row = row.
		Set("id", id).
		Set("quant", r.Quant).
		Set("ctx", ctx).
		Set("tier_1", t1).
		Set("tier_2", t2).
		Set("tier_3", t3).
		Set("tok_s", tok).
		Set("free_mem", free)
	if !human {
		row = row.Set("verdict", verdictLabel(r)) // agents read words, people read the mark after the ID
		source := r.Source
		if source == "" {
			source = "-"
		}
		row = row.Set("source", source)
	}
	return row
}

// minutesLabel keeps one decimal under 10 minutes (4.9m) and none above (13m).
func minutesLabel(min float64) string {
	if min < 9.95 {
		return fmt.Sprintf("%.1fm", min)
	}
	return fmt.Sprintf("%.0fm", min)
}

// verdictLabel is the overall result of a measured model, so a row whose times
// look fine is still not read as a pass when it failed (for example on swap
// growth). A run that passed only with looser limits than pill's is a fail:
// there is no in-between verdict. Models nobody measured print "-".
func verdictLabel(r catalog.Row) string {
	if r.Measurement == nil {
		return "-"
	}
	switch r.Status {
	case catalog.StatusRecommended:
		return "ok"
	case catalog.StatusRelaxed, catalog.StatusFailed:
		return "FAIL"
	}
	return "-"
}

// verdictMark is the verdict as one character for the human table: a green ✓
// passed, and a red ✗ only on a row whose Tier 1 reads fail, so the cross sits
// next to the word it confirms. A run that passed Tier 1 but failed on another
// limit (swap growth, the Tier 2 time cap) gets no mark: it is not a ✓, and
// its failure is not shown as one. The agent verdict still says FAIL.
// Unmeasured rows get nothing.
func verdictMark(r catalog.Row, p *output.Printer) string {
	switch verdictLabel(r) {
	case "ok":
		return p.Paint("32", "✓")
	case "FAIL":
		if t1 := r.Measurement.Tier1; t1 == "fail" || t1 == "relaxed" {
			return p.Paint("31", "✗")
		}
	}
	return ""
}

// tier3Label joins the Tier 3 scores out of 100 in the fixed catalog.Tier3Order,
// with the three landing pages in brackets: 88|91|40|90|(93,95,98). A task
// that was not run prints "-", and "-" alone when Tier 3 did not run at all.
func tier3Label(m *catalog.Measurement) string {
	if len(m.Tier3) == 0 {
		return "-"
	}
	score := map[string]string{}
	for _, t := range m.Tier3 {
		score[t.Task] = strconv.FormatFloat(math.Round(t.Median), 'f', 0, 64)
	}
	var parts []string
	for _, group := range catalog.Tier3Order {
		var cells []string
		for _, task := range group {
			v, ok := score[task]
			if !ok {
				v = "-"
			}
			cells = append(cells, v)
		}
		if len(cells) == 1 {
			parts = append(parts, cells[0])
		} else {
			parts = append(parts, "("+strings.Join(cells, ",")+")")
		}
	}
	return strings.Join(parts, "|")
}
