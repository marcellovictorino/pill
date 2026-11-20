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
			t2full = t2full && run.Tier2Total > 0 && run.Tier2Score == run.Tier2Total
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
	case t1 && t2full && r.Passed: // the overall verdict decides, not just the scores (memory can fail a 22/22 run)
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

// PersonalRow is one line of `pill bench summary --personal`: the latest
// personal result of a model beside its latest bare result, the baseline. A
// personal result is informational, so there is no pass or fail here.
type PersonalRow struct {
	Model     string
	SetupHash string
	Runs      int
	ToolCheck string  // runs whose Pi tool-calling turn answered correctly, e.g. "3/3"
	ToolSecs  float64 // mean length of that turn
	ExtraSecs string  // ToolSecs minus the bare mean, "-" when the bare run did not record it
	Tier2     string  // lowest Tier 2 score of the runs, "-" when none ran
	Tier2Secs float64 // longest Tier 2 time
	BareTier2 string  // the same for the latest bare result, "-" when there is none
	BareSecs  float64
	Date      string
}

// toolTurnPassed reports whether the run's Pi tool-calling turn answered correctly.
func toolTurnPassed(run config.RunResult) bool {
	for _, c := range run.Tier1Checks {
		if c.Name == "Pi tool-calling turn returns the right answer" {
			return c.Pass
		}
	}
	return false
}

// tier2Of is the lowest Tier 2 score and longest Tier 2 time of a result.
func tier2Of(r config.Result) (score string, secs float64) {
	score = "-"
	min, total := -1, 0
	for _, run := range r.Runs {
		if !run.Tier2Ran {
			continue
		}
		if min < 0 || run.Tier2Score < min {
			min, total = run.Tier2Score, run.Tier2Total
		}
		secs = max(secs, run.Tier2Seconds)
	}
	if min >= 0 {
		score = scoreText(min, total)
	}
	return score, secs
}

func meanToolSecs(r config.Result) float64 {
	var sum float64
	for _, run := range r.Runs {
		sum += run.Tier1ToolSeconds
	}
	if len(r.Runs) == 0 {
		return 0
	}
	return sum / float64(len(r.Runs))
}

// SummarizePersonal pairs the latest personal result of each model with the
// latest bare result of the same model, models in alphabetical order.
func SummarizePersonal(personal []config.PersonalResult, bare []config.Result) []PersonalRow {
	latestBare := map[string]config.Result{}
	for _, r := range bare { // oldest first
		latestBare[r.Model] = r
	}
	latest := map[string]config.PersonalResult{}
	for _, r := range personal {
		if cur, ok := latest[r.Model]; !ok || !r.Time.Before(cur.Time) {
			latest[r.Model] = r
		}
	}
	var rows []PersonalRow
	for model, r := range latest {
		row := PersonalRow{Model: model, SetupHash: r.SetupHash, Runs: len(r.Runs), Date: r.Time.Format("2006-01-02"),
			ToolSecs: meanToolSecs(r.Result), ExtraSecs: "-", BareTier2: "-"}
		ok := 0
		for _, run := range r.Runs {
			if toolTurnPassed(run) {
				ok++
			}
		}
		row.ToolCheck = strconv.Itoa(ok) + "/" + strconv.Itoa(len(r.Runs))
		row.Tier2, row.Tier2Secs = tier2Of(r.Result)
		if b, has := latestBare[model]; has {
			row.BareTier2, row.BareSecs = tier2Of(b)
			if bs := meanToolSecs(b); bs > 0 {
				row.ExtraSecs = strconv.FormatFloat(row.ToolSecs-bs, 'f', 1, 64)
			}
		}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Model < rows[j].Model })
	return rows
}
