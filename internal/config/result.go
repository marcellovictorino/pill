package config

import "time"

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
	Tier1Pass     bool    `json:"tier1_pass"`
	Tier1Seconds  float64 `json:"tier1_seconds"`
	Tier1Checks   []Check `json:"tier1_checks,omitempty"`
	Tier2Ran      bool    `json:"tier2_ran"`
	Tier2Score    int     `json:"tier2_score"`
	Tier2Total    int     `json:"tier2_total"`
	Tier2Seconds  float64 `json:"tier2_seconds"`
	Tier2Checks   []Check `json:"tier2_checks,omitempty"`
	Tier2TimedOut bool    `json:"tier2_timed_out,omitempty"`
	Tier2Repeats  int     `json:"tier2_repeated_calls,omitempty"` // longest run of identical tool calls
	MinFreePct    float64 `json:"min_free_pct"`
	SwapGrowthMB  float64 `json:"swap_growth_mb"`
	GPUBusyMin    float64 `json:"gpu_busy_min"`
	GenTokPerSec  float64 `json:"gen_tok_per_sec"`
	PromptTokens  int     `json:"prompt_tokens"`
	GenTokens     int     `json:"gen_tokens"`
	Passed        bool    `json:"passed"`
	Reason        string  `json:"reason,omitempty"`
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
