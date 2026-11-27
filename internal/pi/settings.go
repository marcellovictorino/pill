package pi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// SettingsPath is the settings.json inside dir.
func SettingsPath(dir string) string { return filepath.Join(dir, "settings.json") }

// modelRef is how Pi names a model in its per-model settings.
func modelRef(id string) string { return ProviderKey + "/" + id }

// CompactionReserve is the reserveTokens that makes Pi start compacting at 80%
// of a window of ctx tokens. Pi has no percentage setting: it compacts once
// the context passes the window minus reserveTokens, so 20% is held back.
func CompactionReserve(ctx int) int { return ctx / 5 }

// compactionOverrides sets compaction.modelOverrides["pill/<id>"].reserveTokens
// for each model in doc and drops pill/ entries of models no longer listed.
// A per-model override wins over a general compaction.reserveTokens, so a rule
// the user set for other models (say a large Claude window) is neither read nor
// changed. Keys outside "pill/" are never touched.
func compactionOverrides(doc *ordered, models []Model) error {
	var compaction, overrides ordered
	if raw, ok := doc.vals["compaction"]; ok {
		if err := json.Unmarshal(raw, &compaction); err != nil {
			return fmt.Errorf("\"compaction\" is not an object: %w", err)
		}
	}
	if raw, ok := compaction.vals["modelOverrides"]; ok {
		if err := json.Unmarshal(raw, &overrides); err != nil {
			return fmt.Errorf("\"compaction.modelOverrides\" is not an object: %w", err)
		}
	}
	want := map[string]int{}
	for _, m := range models {
		if m.Ctx > 0 {
			want[modelRef(m.ID)] = CompactionReserve(m.Ctx)
		}
	}
	for _, k := range append([]string(nil), overrides.keys...) {
		if _, keep := want[k]; strings.HasPrefix(k, ProviderKey+"/") && !keep {
			overrides.del(k)
		}
	}
	for _, m := range models {
		k := modelRef(m.ID)
		n, ok := want[k]
		if !ok {
			continue
		}
		var entry ordered
		if raw, had := overrides.vals[k]; had {
			if err := json.Unmarshal(raw, &entry); err != nil {
				return fmt.Errorf("compaction.modelOverrides[%q] is not an object: %w", k, err)
			}
		}
		entry.set("reserveTokens", json.RawMessage(strconv.Itoa(n)))
		eb, _ := entry.MarshalJSON()
		overrides.set(k, eb)
	}
	if len(overrides.keys) == 0 {
		if _, had := compaction.vals["modelOverrides"]; !had {
			return nil // nothing of ours, nothing to create
		}
	}
	ob, _ := overrides.MarshalJSON()
	compaction.set("modelOverrides", ob)
	cb, _ := compaction.MarshalJSON()
	doc.set("compaction", cb)
	return nil
}

// MergeCompaction writes pill's per-model compaction rule into the Pi
// settings.json at path: compaction starts at 80% of each model's window. It
// leaves every other key untouched, like Merge: it copies an existing file to
// settings.json.bak before changing it, writes atomically, refuses a file that
// is not valid JSON, and reports whether the file changed.
func MergeCompaction(path string, models []Model) (changed bool, err error) {
	var doc ordered
	existing, readErr := os.ReadFile(path)
	switch {
	case errors.Is(readErr, fs.ErrNotExist):
		if len(models) == 0 {
			return false, nil
		}
	case readErr != nil:
		return false, readErr
	default:
		if err := json.Unmarshal(existing, &doc); err != nil {
			return false, fmt.Errorf("%s is not valid JSON, refusing to modify it: %w", path, err)
		}
	}
	if err := compactionOverrides(&doc, models); err != nil {
		return false, fmt.Errorf("%s: %w", path, err)
	}
	if readErr != nil && len(doc.keys) == 0 {
		return false, nil
	}
	raw, _ := doc.MarshalJSON()
	var out bytes.Buffer
	if err := json.Indent(&out, raw, "", "  "); err != nil {
		return false, err
	}
	out.WriteByte('\n')
	return writeWithBackup(path, existing, readErr == nil, out.Bytes())
}

// WriteBenchSettings writes the throwaway Pi folder's settings.json: only the
// compaction rule for the model under test, so a benchmark compacts the way a
// day-to-day pill session does and nothing of the user's own settings leaks in.
func WriteBenchSettings(dir string, m Model) error {
	_, err := MergeCompaction(SettingsPath(dir), []Model{m})
	return err
}
