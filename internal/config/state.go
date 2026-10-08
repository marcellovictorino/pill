package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
)

// Gate states, in lifecycle order. "pulled" (a GGUF with no entry) is derived
// from the filesystem and never stored.
const (
	StateUnverified = "unverified" // servable, but not proven on this machine
	StatePassed     = "passed"     // benchmarked here and met every threshold
	StateFailed     = "failed"     // benchmarked here and did not; never offered to Pi
)

// ModelState is the machine-local status of one entry. It lives in
// ~/.pill/results.json, not in the stowed config, because a pass on one Mac
// says nothing about another.
type ModelState struct {
	State string `json:"state"`
	// InPi is true when the entry is registered in Pi's models.json: after
	// `pill add --unverified`, `pill sync`, or a bench pass.
	InPi bool `json:"in_pi"`
	// PresetHash and GGUFSize fingerprint what was judged. If the rendered
	// preset section or the file changes, the entry falls back to unverified.
	PresetHash string  `json:"preset_hash,omitempty"`
	GGUFSize   int64   `json:"gguf_size,omitempty"`
	GGUFSHA256 string  `json:"gguf_sha256,omitempty"`
	Latest     *Result `json:"latest,omitempty"`
}

// FileInfo remembers where a downloaded GGUF came from, so `pill add` and
// `pill sync` can tie a file back to its Hugging Face source.
type FileInfo struct {
	Repo   string `json:"repo,omitempty"`
	Quant  string `json:"quant,omitempty"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256,omitempty"`
}

// Results is the whole results.json file.
type Results struct {
	Version int                    `json:"version"`
	Models  map[string]*ModelState `json:"models"`
	Files   map[string]FileInfo    `json:"files,omitempty"`
}

// Get returns the state for name, or nil.
func (r *Results) Get(name string) *ModelState { return r.Models[name] }

// Ensure returns the state for name, creating it as unverified.
func (r *Results) Ensure(name string) *ModelState {
	if st, ok := r.Models[name]; ok {
		return st
	}
	st := &ModelState{State: StateUnverified}
	r.Models[name] = st
	return st
}

// LoadResults reads results.json; a missing file is an empty set.
func LoadResults(p Paths) (*Results, error) {
	r := &Results{Version: 1, Models: map[string]*ModelState{}, Files: map[string]FileInfo{}}
	data, err := os.ReadFile(p.ResultsFile())
	if errors.Is(err, fs.ErrNotExist) {
		return r, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, r); err != nil {
		return nil, fmt.Errorf("parse %s: %w", p.ResultsFile(), err)
	}
	if r.Models == nil {
		r.Models = map[string]*ModelState{}
	}
	if r.Files == nil {
		r.Files = map[string]FileInfo{}
	}
	return r, nil
}

// SaveResults writes results.json atomically.
func SaveResults(p Paths, r *Results) error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return WriteFileAtomic(p.ResultsFile(), append(data, '\n'), 0o644)
}

// EffectiveState is the state to show and act on. It starts from the stored
// state but demotes to unverified when the evidence no longer matches: the
// preset section changed, or the GGUF has a different size.
func EffectiveState(st *ModelState, presetHash string, fileSize int64) string {
	if st == nil || st.State == "" {
		return StateUnverified
	}
	if st.State == StateUnverified {
		return StateUnverified
	}
	if st.PresetHash != "" && presetHash != "" && st.PresetHash != presetHash {
		return StateUnverified
	}
	if st.GGUFSize != 0 && fileSize != 0 && st.GGUFSize != fileSize {
		return StateUnverified
	}
	return st.State
}
