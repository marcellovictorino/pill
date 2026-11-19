package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"time"
)

// Environment records what a benchmark ran on, so results stay comparable.
type Environment struct {
	LlamaCpp string `json:"llama_cpp"` // e.g. "0.6.0 (build 11429, commit d81235049)"
	Pi       string `json:"pi"`
	Pill     string `json:"pill"`
	MacOS    string `json:"macos"`
	RAMGB    int    `json:"ram_gb"`
}

// Thresholds are the gate values a run was judged against. They are flags on
// `pill bench run` and recorded in every result.
type Thresholds struct {
	Tier1MaxSeconds float64 `json:"tier1_max_seconds"`
	MinFreePct      float64 `json:"min_free_pct"`
	Tier2MaxMinutes float64 `json:"tier2_max_minutes"`
}

// Check is one named pass/fail observation.
type Check struct {
	Name   string `json:"name"`
	Pass   bool   `json:"pass"`
	Detail string `json:"detail,omitempty"`
}

// RunResult is one benchmark run of one model.
type RunResult struct {
	Tier1Pass    bool    `json:"tier1_pass"`
	Tier1Seconds float64 `json:"tier1_seconds"`
	Tier1Checks  []Check `json:"tier1_checks,omitempty"`
	// Tier1ToolSeconds is how long the Pi tool-calling turn took; the timed
	// request bypasses Pi, so only this turn shows what a Pi setup costs.
	Tier1ToolSeconds float64 `json:"tier1_tool_seconds,omitempty"`
	Tier2Ran         bool    `json:"tier2_ran"`
	Tier2Score       int     `json:"tier2_score"`
	Tier2Total       int     `json:"tier2_total"`
	Tier2Seconds     float64 `json:"tier2_seconds"`
	Tier2Checks      []Check `json:"tier2_checks,omitempty"`
	Tier2TimedOut    bool    `json:"tier2_timed_out,omitempty"`
	Tier2Repeats     int     `json:"tier2_repeated_calls,omitempty"` // longest run of identical tool calls
	MinFreePct       float64 `json:"min_free_pct"`
	SwapGrowthMB     float64 `json:"swap_growth_mb"`
	GPUBusyMin       float64 `json:"gpu_busy_min"`
	GenTokPerSec     float64 `json:"gen_tok_per_sec"`
	PromptTokens     int     `json:"prompt_tokens"`
	GenTokens        int     `json:"gen_tokens"`
	Passed           bool    `json:"passed"`
	Reason           string  `json:"reason,omitempty"`
}

// Result is the outcome of `pill bench run`: every run plus the verdict.
type Result struct {
	ID         string      `json:"id"` // directory name under ~/.pill/benchmark
	Model      string      `json:"model"`
	Time       time.Time   `json:"time"`
	Passed     bool        `json:"passed"`
	Env        Environment `json:"env"`
	Thresholds Thresholds  `json:"thresholds"`
	Runs       []RunResult `json:"runs"`
}

// PersonalResult is a benchmark run with the user's own Pi setup. It is
// informational: Passed is never set, and it never changes a model's state.
type PersonalResult struct {
	Result
	Personal  bool   `json:"personal"`   // marker: this record must never be committed or catalogued
	SetupHash string `json:"setup_hash"` // fingerprint of the copied Pi setup, so a changed setup shows later
}

// LoadPersonal reads personal/results.json (empty when there is none yet).
func LoadPersonal(p Paths) ([]PersonalResult, error) {
	data, err := os.ReadFile(p.PersonalResults())
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	var out []PersonalResult
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("%s is not valid JSON: %w", p.PersonalResults(), err)
	}
	return out, nil
}

// AppendPersonal adds one record to personal/results.json. The caller holds
// the benchmark lock, so two runs cannot interleave; this never touches
// results.json.
func AppendPersonal(p Paths, r PersonalResult) error {
	all, err := LoadPersonal(p)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(append(all, r), "", "  ")
	if err != nil {
		return err
	}
	return WriteFileAtomic(p.PersonalResults(), append(data, '\n'), 0o600)
}
