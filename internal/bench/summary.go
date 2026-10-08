package bench

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"

	"github.com/marcellovictorino/pill/internal/config"
)

// SummaryRow is one line of `pill bench summary`.
type SummaryRow struct {
	Rank        int
	Model       string
	Tier1       string // pass / fail
	Tier2Score  string // "22/22", or "-" when Tier 2 did not run
	Tier2Secs   float64
	MinFreePct  float64
	SwapGrowthM float64
	TokPerSec   float64
	Runs        int
	Build       string // llama.cpp build number
	Date        string
	Passed      bool
	OlderBuild  bool // benched on a different llama.cpp than the one installed now

	group      int
	tier2Score int
}

// LoadResults reads every result.json under dir (~/.pill/benchmark).
func LoadResults(dir string) ([]config.Result, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*", "result.json"))
	if err != nil {
		return nil, err
	}
	var results []config.Result
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var r config.Result
		if json.Unmarshal(data, &r) == nil {
			results = append(results, r)
		}
	}
	sort.Slice(results, func(i, j int) bool { return results[i].Time.Before(results[j].Time) })
	return results, nil
}

var buildRe = regexp.MustCompile(`build (\d+)`)

// BuildOf extracts the llama.cpp build number from a version string.
func BuildOf(version string) string {
	if m := buildRe.FindStringSubmatch(version); m != nil {
		return m[1]
	}
	return ""
}

// Summarize ranks results. By default only the latest result per model is
// kept; all keeps every result. keep (when non-nil) limits it to models for
// which keep returns true. currentBuild is the installed llama.cpp build,
// used to flag results measured on another build.
//
// Ranking: passed both tiers, then passed Tier 1 only, then failed. Within a
// group: Tier 2 score (higher first), minimum free memory (higher first),
// Tier 2 time (lower first).
func Summarize(results []config.Result, all bool, currentBuild string, keep func(model string) bool) []SummaryRow {
	if !all {
		latest := map[string]config.Result{}
		for _, r := range results { // results are oldest first
			latest[r.Model] = r
		}
		results = results[:0:0]
		for _, r := range latest {
			results = append(results, r)
		}
	}
	var rows []SummaryRow
	for _, r := range results {
		if keep != nil && !keep(r.Model) {
			continue
		}
		rows = append(rows, summarizeOne(r, currentBuild))
	}
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		switch {
		case a.group != b.group:
			return a.group < b.group
		case a.tier2Score != b.tier2Score:
			return a.tier2Score > b.tier2Score
		case a.MinFreePct != b.MinFreePct:
			return a.MinFreePct > b.MinFreePct
		case a.Tier2Secs != b.Tier2Secs:
			return a.Tier2Secs < b.Tier2Secs
		}
		return a.Date > b.Date
	})
	for i := range rows {
		rows[i].Rank = i + 1
	}
	return rows
}

func summarizeOne(r config.Result, currentBuild string) SummaryRow {
	row := SummaryRow{Model: r.Model, Runs: len(r.Runs), Passed: r.Passed, Date: r.Time.Format("2006-01-02"),
		Build: BuildOf(r.Env.LlamaCpp), Tier1: "pass", Tier2Score: "-", MinFreePct: 100}
	t1, t2full, t2ran := true, true, false
	minScore, minTotal := -1, 0
	var tok float64
	for _, run := range r.Runs {
		t1 = t1 && run.Tier1Pass
		if run.Tier2Ran {
			t2ran = true
			t2full = t2full && run.Tier2Total > 0 && run.Tier2Score == run.Tier2Total && !run.Tier2TimedOut
			if minScore < 0 || run.Tier2Score < minScore {
				minScore, minTotal = run.Tier2Score, run.Tier2Total
			}
			row.Tier2Secs = max(row.Tier2Secs, run.Tier2Seconds)
		} else {
			t2full = false
		}
		row.MinFreePct = min(row.MinFreePct, run.MinFreePct)
		row.SwapGrowthM = max(row.SwapGrowthM, run.SwapGrowthMB)
		tok += run.GenTokPerSec
	}
	if len(r.Runs) > 0 {
		row.TokPerSec = tok / float64(len(r.Runs))
	}
	if !t1 {
		row.Tier1 = "fail"
	}
	if t2ran {
		row.Tier2Score = scoreText(minScore, minTotal)
		row.tier2Score = minScore
	}
	switch {
	case t1 && t2full:
		row.group = 0
	case t1:
		row.group = 1
	default:
		row.group = 2
	}
	row.OlderBuild = currentBuild != "" && row.Build != "" && row.Build != currentBuild
	return row
}

func scoreText(score, total int) string {
	return strconv.Itoa(score) + "/" + strconv.Itoa(total)
}
