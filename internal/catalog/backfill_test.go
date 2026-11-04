package catalog

import (
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// reportRows reads the markdown tables of an evals report: first-cell text ->
// cells. The reports are the source the catalog records were converted from.
func reportRows(t *testing.T, file string) map[string][]string {
	t.Helper()
	b, err := os.ReadFile("../../" + file)
	if err != nil {
		t.Fatal(err)
	}
	rows := map[string][]string{}
	for _, line := range strings.Split(string(b), "\n") {
		if !strings.HasPrefix(line, "| ") {
			continue
		}
		var cells []string
		for _, c := range strings.Split(strings.Trim(line, "| "), " | ") {
			cells = append(cells, strings.TrimSpace(c))
		}
		rows[cells[0]] = cells
	}
	return rows
}

func allMeasurements(c *Catalog) []Measurement {
	var out []Measurement
	for _, e := range c.Entries() {
		out = append(out, e.Variant.Measurements...)
	}
	for _, tr := range c.Tried {
		out = append(out, tr.Measurements...)
	}
	return out
}

// has reports whether the cell holds v as a whole number (so 5 is not found
// inside 25).
func has(cell string, v float64) bool {
	return regexp.MustCompile(`(^|[^0-9.])` + regexp.QuoteMeta(strconv.FormatFloat(v, 'f', -1, 64)) + `([^0-9]|$)`).MatchString(cell)
}

// Intent (PILL-55 A/B check 2): the records backfilled from
// tried-and-rejected.md match its table row by row on Tier 1 time, Tier 2
// score, decode speed, free memory and swap, and every model row has a record.
// A record that drifts from its report, or whose row was renamed, fails here.
func TestBackfilledRecordsMatchTheReportTable(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	rows := reportRows(t, "evals/reports/tried-and-rejected.md")
	seen := map[string]bool{}
	for _, m := range allMeasurements(c) {
		if m.SourceRow == "" {
			continue
		}
		row, ok := rows[m.SourceRow]
		if !ok || len(row) != 7 {
			t.Errorf("source_row %q is not a model row of tried-and-rejected.md", m.SourceRow)
			continue
		}
		seen[m.SourceRow] = true
		t1, t2, tok, free, issues := row[2], row[3], row[4], row[5], row[6]
		if m.Tier1Seconds > 0 && !has(t1, m.Tier1Seconds) {
			t.Errorf("%s: tier1_seconds %v is not in %q", m.SourceRow, m.Tier1Seconds, t1)
		}
		if m.Tier2Total > 0 && !strings.Contains(t2, strconv.Itoa(m.Tier2Score)+"/"+strconv.Itoa(m.Tier2Total)) {
			t.Errorf("%s: score %d/%d is not in %q", m.SourceRow, m.Tier2Score, m.Tier2Total, t2)
		}
		if m.TokPerS > 0 && !has(tok, m.TokPerS) {
			t.Errorf("%s: tok_per_s %v is not in %q", m.SourceRow, m.TokPerS, tok)
		}
		if m.MinFreePct != nil && !has(free, *m.MinFreePct) {
			t.Errorf("%s: min_free_pct %v is not in %q", m.SourceRow, *m.MinFreePct, free)
		}
		// The report writes swap in GB or MB, in the Issues or Result cell.
		if m.SwapMB != nil && *m.SwapMB > 0 && !has(issues+" "+row[1], *m.SwapMB/1000) && !has(issues, *m.SwapMB) {
			t.Errorf("%s: swap_mb %v is not in %q", m.SourceRow, *m.SwapMB, issues)
		}
	}
	for name, row := range rows {
		if len(row) == 7 && name != "Model" && !seen[name] {
			t.Errorf("report row %q has no catalog record", name)
		}
	}
}

// Intent (PILL-55 A/B check 2): MiniCPM's Tier 3 record equals its report
// table: six runs per task, median and range.
func TestMiniCPMTier3MatchesItsReport(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	e, _ := c.Resolve("minicpm-v4-7")
	var m *Measurement
	for i := range e.Variant.Measurements {
		if e.Variant.Measurements[i].Chip == "Apple M4" {
			m = &e.Variant.Measurements[i]
		}
	}
	if m == nil || len(m.Tier3) != 7 {
		t.Fatalf("MiniCPM needs 7 Tier 3 tasks on the M4: %+v", m)
	}
	rows := reportRows(t, "evals/reports/minicpm-v-4-7-35b-a3b.md")
	for _, task := range m.Tier3 {
		row := rows["`"+task.Task+"`"]
		if row == nil && task.Task == "py-insight" {
			row = rows["`py-insight` (automated)"]
		}
		if len(row) < 4 {
			t.Errorf("no report row for %s", task.Task)
			continue
		}
		if strconv.Itoa(task.Runs) != row[1] || !has(row[2], task.Median) || !has(row[3], task.Min) || !has(row[3], task.Max) {
			t.Errorf("%s: record %+v does not match report row %v", task.Task, task, row)
		}
	}
}
