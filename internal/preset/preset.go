// Package preset generates the models.ini file that llama-server's router
// mode reads: shared defaults plus one section per pill model.
package preset

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/marcellovictorino/pill/internal/config"
)

// header holds the options every model shares ([*] is llama-server's
// "applies to all sections" marker).
const header = `version = 1

[*]
jinja = true
ngl = 99
flash-attn = on
cache-type-k = q8_0
cache-type-v = q8_0
np = 1
cache-ram = 512
ctx-checkpoints = 4
`

// canonicalOrder keeps well-known sampling keys in a stable, readable order;
// any other keys follow alphabetically.
var canonicalOrder = []string{"temp", "top-p", "top-k", "min-p"}

// Section renders one model's [name] section. The GGUF path is made absolute
// (llama-server resolves relative paths against its own working directory).
func Section(m config.Model, modelsDir string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[%s]\n", m.Name)
	fmt.Fprintf(&b, "model = %s\n", filepath.Join(modelsDir, m.File))
	fmt.Fprintf(&b, "c = %d\n", m.Ctx)
	for _, k := range orderedKeys(m.Preset) {
		fmt.Fprintf(&b, "%s = %s\n", k, formatValue(m.Preset[k]))
	}
	return b.String()
}

// Render builds the complete models.ini for the given entries.
func Render(models []config.Model, modelsDir string) string {
	var b strings.Builder
	b.WriteString(header)
	for _, m := range models {
		b.WriteString("\n")
		b.WriteString(Section(m, modelsDir))
	}
	return b.String()
}

// SectionHash fingerprints the rendered section; the bench gate stores it so
// a pass applies only to the exact settings that were measured.
func SectionHash(m config.Model, modelsDir string) string {
	sum := sha256.Sum256([]byte(Section(m, modelsDir)))
	return hex.EncodeToString(sum[:8])
}

// HashOf fingerprints the whole file, used to notice when a running router
// was started from an older models.ini.
func HashOf(ini string) string {
	sum := sha256.Sum256([]byte(ini))
	return hex.EncodeToString(sum[:8])
}

func orderedKeys(p map[string]any) []string {
	var keys []string
	for _, k := range canonicalOrder {
		if _, ok := p[k]; ok {
			keys = append(keys, k)
		}
	}
	var rest []string
	for k := range p {
		if !contains(canonicalOrder, k) {
			rest = append(rest, k)
		}
	}
	sort.Strings(rest)
	return append(keys, rest...)
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// formatValue prints TOML values the way llama-server's INI parser expects.
// Floats keep a ".0" so "1.0" does not turn into "1".
func formatValue(v any) string {
	switch x := v.(type) {
	case float64:
		s := strconv.FormatFloat(x, 'f', -1, 64)
		if !strings.ContainsAny(s, ".eE") {
			s += ".0"
		}
		return s
	case bool:
		return strconv.FormatBool(x)
	default:
		return fmt.Sprint(x)
	}
}
