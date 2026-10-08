package config

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sort"

	"github.com/BurntSushi/toml"
)

// Model is one entry in models.toml: a named way to serve a GGUF file.
// One GGUF can back several entries with different context sizes.
type Model struct {
	Name      string `toml:"name"`
	Repo      string `toml:"repo,omitempty"`  // Hugging Face repo, so another Mac can download it
	File      string `toml:"file"`            // GGUF file name inside the models dir
	Quant     string `toml:"quant,omitempty"` // informational
	Ctx       int    `toml:"ctx"`             // llama-server context size in tokens
	Reasoning bool   `toml:"reasoning"`
	MaxTokens int    `toml:"max_tokens"`
	// Preset holds extra llama-server options (sampling etc.), written as-is
	// into the generated models.ini section.
	Preset map[string]any `toml:"preset,omitempty"`
}

// ModelsFile is the on-disk shape of models.toml ([[model]] tables).
type ModelsFile struct {
	Models []Model `toml:"model"`
}

// Find returns the entry called name.
func (m *ModelsFile) Find(name string) (*Model, bool) {
	for i := range m.Models {
		if m.Models[i].Name == name {
			return &m.Models[i], true
		}
	}
	return nil, false
}

// Upsert adds or replaces an entry and keeps the list sorted by name so the
// file diffs cleanly in dotfiles.
func (m *ModelsFile) Upsert(model Model) {
	if cur, ok := m.Find(model.Name); ok {
		*cur = model
	} else {
		m.Models = append(m.Models, model)
	}
	sort.Slice(m.Models, func(i, j int) bool { return m.Models[i].Name < m.Models[j].Name })
}

// Remove deletes an entry and reports whether it existed.
func (m *ModelsFile) Remove(name string) bool {
	for i := range m.Models {
		if m.Models[i].Name == name {
			m.Models = append(m.Models[:i], m.Models[i+1:]...)
			return true
		}
	}
	return false
}

// LoadModels reads models.toml; a missing file is an empty list.
func LoadModels(p Paths) (*ModelsFile, error) {
	var mf ModelsFile
	if _, err := toml.DecodeFile(p.ModelsToml(), &mf); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("read %s: %w", p.ModelsToml(), err)
	}
	return &mf, nil
}

const modelsHeader = "# Models pill should serve. Managed by `pill add`, `pill rm` and `pill sync`;\n# hand edits are fine, but comments are not preserved when pill rewrites it.\n\n"

// SaveModels writes models.toml atomically.
func SaveModels(p Paths, mf *ModelsFile) error {
	var buf bytes.Buffer
	buf.WriteString(modelsHeader)
	if err := toml.NewEncoder(&buf).Encode(mf); err != nil {
		return err
	}
	return WriteFileAtomic(p.ModelsToml(), buf.Bytes(), 0o644)
}

// FileExists reports whether path is an existing regular file.
func FileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.Mode().IsRegular()
}
