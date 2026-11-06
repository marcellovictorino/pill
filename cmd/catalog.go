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
				models = append(models, catalogRow(r, a.printer.Human, isDefault))
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
			out := output.Obj{}.Set("chip", chip).Set("ram_gb", dev.RAMGB)
			if !a.printer.Human && defaultID != "" {
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
func catalogRow(r catalog.Row, human, isDefault bool) output.Obj {
	m := r.Measurement
	id := r.ID
	if human && isDefault {
		id += "*"
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
		}
		if m.Tier2Total > 0 {
			t2 = fmt.Sprintf("%d/%d", m.Tier2Score, m.Tier2Total)
		}
		if m.TokPerS > 0 {
			tok = m.TokPerS
		}
		if m.MinFreePct != nil {
			free = fmt.Sprintf("%g%%", *m.MinFreePct)
		}
		t3 = tier3Label(m)
	}
	row := output.Obj{}.
		Set("id", id).
		Set("quant", r.Quant).
		Set("ctx", ctx).
		Set("tier_1", t1).
		Set("tier_2", t2).
		Set("tier_3", t3).
		Set("tok_s", tok).
		Set("free_mem", free)
	if !human {
		source := r.Source
		if source == "" {
			source = "-"
		}
		row = row.Set("source", source)
	}
	return row
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
