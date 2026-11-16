package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
)

// Gate states, in lifecycle order. "pulled" (a GGUF with no entry) is derived
// from the filesystem and never stored. Whether Pi is offered a model follows
// from its state alone (see OfferedToPi).
const (
	StateCandidate  = "candidate"  // has an entry, not offered to Pi: being benchmarked, or a failed verdict no longer applies
	StateUnverified = "unverified" // offered to Pi, but not proven on this machine
	StatePassed     = "passed"     // benchmarked here and met every threshold
	StateFailed     = "failed"     // benchmarked here and did not; never offered to Pi
)

// Gate events: what a command does to an entry.
const (
	EventRegister = "register" // pill add --unverified, or pill sync registering a file it found or downloaded
	EventPass     = "pass"     // a benchmark met every threshold
	EventFail     = "fail"     // a benchmark missed one, and any demotion of a pass was confirmed
)

// Next is the gate's transition table: the state an entry moves to when event
// happens in state. Settings drift is not an event: EffectiveState derives it.
func Next(state, event string) string {
	switch event {
	case EventRegister:
		if state == StatePassed {
			return StatePassed // registering again does not undo a pass
		}
		return StateUnverified // the explicit override of a failed benchmark, too
	case EventPass:
		return StatePassed
	case EventFail:
		return StateFailed
	}
	return state
}

// OfferedToPi reports whether an entry in state belongs in Pi's models.json.
func OfferedToPi(state string) bool {
	return state == StateUnverified || state == StatePassed
}

// ModelState is the machine-local status of one entry. It lives in
// ~/.pill/results.json, not in the stowed config, because a pass on one Mac
// says nothing about another.
type ModelState struct {
	State string `json:"state"`
	// PresetHash and GGUFSize fingerprint what was judged. If the rendered
	// preset section or the file changes, the verdict no longer applies.
	PresetHash string  `json:"preset_hash,omitempty"`
	GGUFSize   int64   `json:"gguf_size,omitempty"`
	GGUFSHA256 string  `json:"gguf_sha256,omitempty"`
	Latest     *Result `json:"latest,omitempty"`
}

// FileInfo remembers where a downloaded GGUF came from, so `pill add` and
// `pill sync` can tie a file back to its Hugging Face source.
type FileInfo struct {
	Repo   string `json:"repo,omitempty"`
	Path   string `json:"path,omitempty"` // the file's path inside the repository (it can sit in a folder)
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

// Ensure returns the state for name, creating it as a candidate.
func (r *Results) Ensure(name string) *ModelState {
	if st, ok := r.Models[name]; ok {
		return st
	}
	st := &ModelState{State: StateCandidate}
	r.Models[name] = st
	return st
}

// resultsVersion 2 dropped the in_pi flag: being offered to Pi follows from the state.
const resultsVersion = 2

// LoadResults reads results.json; a missing file is an empty set.
func LoadResults(p Paths) (*Results, error) {
	r := &Results{Version: resultsVersion, Models: map[string]*ModelState{}, Files: map[string]FileInfo{}}
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
	if r.Version < 2 {
		if err := migrateInPi(r, data); err != nil {
			return nil, fmt.Errorf("parse %s: %w", p.ResultsFile(), err)
		}
	}
	return r, nil
}

// migrateInPi reads version 1's in_pi flag once: an unverified entry Pi was
// not offered is a candidate. The flag is not written back.
func migrateInPi(r *Results, data []byte) error {
	var v1 struct {
		Models map[string]struct {
			InPi bool `json:"in_pi"`
		} `json:"models"`
	}
	if err := json.Unmarshal(data, &v1); err != nil {
		return err
	}
	for name, st := range r.Models {
		if st.State == StateUnverified && !v1.Models[name].InPi {
			st.State = StateCandidate
		}
	}
	r.Version = resultsVersion
	return nil
}

// SaveResults writes results.json atomically.
func SaveResults(p Paths, r *Results) error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if cur, err := os.ReadFile(p.ResultsFile()); err == nil && bytes.Equal(cur, data) {
		return nil // unchanged: do not rewrite
	}
	return WriteFileAtomic(p.ResultsFile(), data, 0o644)
}

// EffectiveState is the state to show and act on. It starts from the stored
// state, but a verdict whose evidence no longer matches (the preset section
// changed, or the GGUF has a different size) no longer applies: a pass falls
// back to unverified and stays offered to Pi, a fail falls back to candidate
// and stays hidden from it.
func EffectiveState(st *ModelState, presetHash string, fileSize int64) string {
	if st == nil || st.State == "" {
		return StateCandidate
	}
	if st.State != StatePassed && st.State != StateFailed {
		return st.State
	}
	drifted := (st.PresetHash != "" && presetHash != "" && st.PresetHash != presetHash) ||
		(st.GGUFSize != 0 && fileSize != 0 && st.GGUFSize != fileSize)
	switch {
	case !drifted:
		return st.State
	case st.State == StatePassed:
		return StateUnverified
	default:
		return StateCandidate
	}
}
