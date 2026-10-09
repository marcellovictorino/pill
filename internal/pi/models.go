// Package pi integrates with the Pi coding agent: it owns one provider key
// ("pill") in Pi's models.json and launches Pi against the router.
package pi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/marcellovictorino/pill/internal/config"
)

// ProviderKey is the only key pill ever writes in Pi's models.json.
const ProviderKey = "pill"

// Model describes one entry for Pi's model list.
type Model struct {
	ID         string
	Reasoning  bool
	Ctx        int
	MaxTokens  int
	Unverified bool
}

// AgentDir is Pi's config directory: PI_CODING_AGENT_DIR (Pi's own override)
// or ~/.pi/agent.
func AgentDir() (string, error) {
	if d := os.Getenv("PI_CODING_AGENT_DIR"); d != "" {
		if strings.HasPrefix(d, "~/") {
			home, _ := os.UserHomeDir()
			d = filepath.Join(home, d[2:])
		}
		return d, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".pi", "agent"), nil
}

// ModelsPath is the models.json inside dir.
func ModelsPath(dir string) string { return filepath.Join(dir, "models.json") }

// The structs below fix the JSON key order of what pill writes. (encoding/json
// writes struct fields in declaration order but map keys alphabetically.)
type providerJSON struct {
	API     string      `json:"api"`
	APIKey  string      `json:"apiKey"`
	BaseURL string      `json:"baseUrl"`
	Compat  compatJSON  `json:"compat"`
	Models  []modelJSON `json:"models"`
}

type compatJSON struct {
	SupportsStore           bool   `json:"supportsStore"`
	SupportsDeveloperRole   bool   `json:"supportsDeveloperRole"`
	SupportsReasoningEffort bool   `json:"supportsReasoningEffort"`
	SupportsUsageInStream   bool   `json:"supportsUsageInStreaming"`
	SupportsStrictMode      bool   `json:"supportsStrictMode"`
	MaxTokensField          string `json:"maxTokensField"`
	ThinkingFormat          string `json:"thinkingFormat"`
}

type modelJSON struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Reasoning     bool     `json:"reasoning"`
	Input         []string `json:"input"`
	ContextWindow int      `json:"contextWindow"`
	MaxTokens     int      `json:"maxTokens"`
}

// BuildProvider renders the provider block for the given models. The
// "(unverified)" marker goes in the display name only: the id never changes,
// so a later bench pass does not break anyone's --model argument.
func BuildProvider(port int, models []Model) json.RawMessage {
	p := providerJSON{
		API:     "openai-completions",
		APIKey:  "local",
		BaseURL: fmt.Sprintf("http://127.0.0.1:%d/v1", port),
		Compat: compatJSON{
			SupportsUsageInStream: true,
			MaxTokensField:        "max_tokens",
			// Gemma 4 toggles thinking per request through the chat template,
			// so Pi's thinking levels act as on/off.
			ThinkingFormat: "qwen-chat-template",
		},
		Models: []modelJSON{},
	}
	for _, m := range models {
		name := m.ID
		if m.Unverified {
			name += " (unverified)"
		}
		p.Models = append(p.Models, modelJSON{
			ID: m.ID, Name: name, Reasoning: m.Reasoning, Input: []string{"text"},
			ContextWindow: m.Ctx, MaxTokens: m.MaxTokens,
		})
	}
	data, _ := json.Marshal(p) // a struct of plain values cannot fail to marshal
	return data
}

// ordered is a JSON object that remembers key order, so rewriting a
// hand-edited file keeps its layout. Values stay raw: parts pill does not own
// are written back byte for byte.
type ordered struct {
	keys []string
	vals map[string]json.RawMessage
}

func (o *ordered) UnmarshalJSON(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	if t, err := dec.Token(); err != nil || t != json.Delim('{') {
		return errors.New("expected a JSON object")
	}
	o.vals = map[string]json.RawMessage{}
	for dec.More() {
		t, err := dec.Token()
		if err != nil {
			return err
		}
		key, _ := t.(string)
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return err
		}
		if _, seen := o.vals[key]; !seen {
			o.keys = append(o.keys, key)
		}
		o.vals[key] = raw
	}
	return nil
}

func (o *ordered) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, k := range o.keys {
		if i > 0 {
			b.WriteByte(',')
		}
		kb, _ := json.Marshal(k)
		b.Write(kb)
		b.WriteByte(':')
		b.Write(o.vals[k])
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

func (o *ordered) set(key string, raw json.RawMessage) {
	if o.vals == nil {
		o.vals = map[string]json.RawMessage{}
	}
	if _, ok := o.vals[key]; !ok {
		o.keys = append(o.keys, key)
	}
	o.vals[key] = raw
}

func (o *ordered) del(key string) {
	if _, ok := o.vals[key]; !ok {
		return
	}
	delete(o.vals, key)
	for i, k := range o.keys {
		if k == key {
			o.keys = append(o.keys[:i], o.keys[i+1:]...)
			break
		}
	}
}

// Merge writes provider under providers.pill in the models.json at path and
// leaves every other key untouched. A nil provider removes pill's key.
//
// Before changing an existing file it copies it to models.json.bak, and the
// write itself is atomic (temp file + rename). It reports whether the file
// changed; an up-to-date file is left alone.
func Merge(path string, provider json.RawMessage) (changed bool, err error) {
	var doc ordered
	existing, readErr := os.ReadFile(path)
	switch {
	case errors.Is(readErr, fs.ErrNotExist):
		if provider == nil {
			return false, nil
		}
	case readErr != nil:
		return false, readErr
	default:
		if err := json.Unmarshal(existing, &doc); err != nil {
			return false, fmt.Errorf("%s is not valid JSON, refusing to modify it: %w", path, err)
		}
	}

	var providers ordered
	if raw, ok := doc.vals["providers"]; ok {
		if err := json.Unmarshal(raw, &providers); err != nil {
			return false, fmt.Errorf("%s: \"providers\" is not an object: %w", path, err)
		}
	}
	if provider == nil {
		if _, had := providers.vals[ProviderKey]; !had {
			return false, nil // nothing of ours to remove
		}
		providers.del(ProviderKey)
	} else {
		providers.set(ProviderKey, provider)
	}
	pb, _ := providers.MarshalJSON()
	doc.set("providers", pb)

	raw, _ := doc.MarshalJSON()
	var out bytes.Buffer
	if err := json.Indent(&out, raw, "", "  "); err != nil {
		return false, err
	}
	out.WriteByte('\n')

	if readErr == nil {
		if bytes.Equal(bytes.TrimSpace(existing), bytes.TrimSpace(out.Bytes())) {
			return false, nil
		}
		if err := config.WriteFileAtomic(path+".bak", existing, 0o600); err != nil {
			return false, fmt.Errorf("back up %s: %w", path, err)
		}
	}
	if err := config.WriteFileAtomic(path, out.Bytes(), 0o600); err != nil {
		return false, err
	}
	return true, nil
}

// ReadProvider returns the pill provider block currently in models.json
// (nil when absent) and the other provider keys found there.
func ReadProvider(path string) (provider json.RawMessage, others []string, err error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	var doc ordered
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, nil, fmt.Errorf("%s is not valid JSON: %w", path, err)
	}
	var providers ordered
	if raw, ok := doc.vals["providers"]; ok {
		if err := json.Unmarshal(raw, &providers); err != nil {
			return nil, nil, err
		}
	}
	for _, k := range providers.keys {
		if k == ProviderKey {
			provider = providers.vals[k]
		} else {
			others = append(others, k)
		}
	}
	return provider, others, nil
}

// ProviderModelIDs lists the model ids inside a provider block.
func ProviderModelIDs(provider json.RawMessage) []string {
	var p struct {
		Models []struct {
			ID string `json:"id"`
		} `json:"models"`
	}
	if json.Unmarshal(provider, &p) != nil {
		return nil
	}
	ids := make([]string, 0, len(p.Models))
	for _, m := range p.Models {
		ids = append(ids, m.ID)
	}
	return ids
}

// OtherProviders maps every provider key except pill's to its baseUrl (empty
// when it has none). doctor uses it to spot a leftover local-router provider.
func OtherProviders(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var doc struct {
		Providers map[string]struct {
			BaseURL string `json:"baseUrl"`
		} `json:"providers"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("%s is not valid JSON: %w", path, err)
	}
	out := map[string]string{}
	for k, p := range doc.Providers {
		if k != ProviderKey {
			out[k] = p.BaseURL
		}
	}
	return out, nil
}

// ProviderBaseURL extracts baseUrl from a provider block.
func ProviderBaseURL(provider json.RawMessage) string {
	var p struct {
		BaseURL string `json:"baseUrl"`
	}
	_ = json.Unmarshal(provider, &p)
	return p.BaseURL
}

// WriteBenchAgentDir creates a throwaway Pi config directory containing only
// the pill provider with one model. Benchmarks run Pi against it (through the
// PI_CODING_AGENT_DIR variable) so the user's own Pi extensions, skills and
// defaults cannot skew the result.
func WriteBenchAgentDir(dir string, port int, m Model) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	m.Unverified = false
	doc := struct {
		Providers map[string]json.RawMessage `json:"providers"`
	}{Providers: map[string]json.RawMessage{ProviderKey: BuildProvider(port, []Model{m})}}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	return config.WriteFileAtomic(ModelsPath(dir), append(data, '\n'), 0o600)
}
