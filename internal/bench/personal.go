package bench

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/marcellovictorino/pill/internal/config"
	"github.com/marcellovictorino/pill/internal/pi"
)

// personalSkip names what a personal run never copies from the top level of
// the user's Pi folder: history and caches. Backups and Hindsight are matched
// by name in skipTop.
var personalSkip = map[string]bool{"sessions": true, "web-search-cache": true}

// skipTop reports whether a top-level entry of the Pi folder stays out of the
// copy. The Hindsight add-on talks to its own memory service, so a copy would
// read and write the user's real memory and let memories leak into the run.
func skipTop(name string) bool {
	low := strings.ToLower(name)
	return personalSkip[name] || strings.Contains(low, ".bak") || strings.HasSuffix(low, "-backup") ||
		(strings.Contains(low, "hindsight") && name != "settings.json")
}

// CopyPersonalPi copies the user's Pi folder src into dest for a personal run,
// without sessions, caches, backups or Hindsight (its entries are also dropped
// from settings.json). src is only read. Links are followed, so nothing in the
// copy can write through to the original, and dangling links are skipped.
func CopyPersonalPi(src, dest string) error {
	if err := copyTree(src, dest, 0, true, false); err != nil {
		return err
	}
	settings := filepath.Join(dest, "settings.json")
	data, err := os.ReadFile(settings)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("%s is not valid JSON: %w", filepath.Join(src, "settings.json"), err)
	}
	for _, key := range []string{"extensions", "packages"} {
		list, ok := cfg[key].([]any)
		if !ok {
			continue
		}
		kept := make([]any, 0, len(list))
		for _, e := range list {
			b, _ := json.Marshal(e)
			if !strings.Contains(strings.ToLower(string(b)), "hindsight") {
				kept = append(kept, e)
			}
		}
		cfg[key] = kept
	}
	out, _ := json.MarshalIndent(cfg, "", "  ")
	return config.WriteFileAtomic(settings, append(out, '\n'), 0o600)
}

// maxCopyDepth stops a link cycle in the user's folder from recursing forever.
const maxCopyDepth = 32

func copyTree(src, dest string, depth int, top, inExtensions bool) error {
	if depth > maxCopyDepth {
		return fmt.Errorf("%s is nested too deeply (a link cycle?)", src)
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		name := e.Name()
		if top && skipTop(name) {
			continue
		}
		if inExtensions && strings.Contains(strings.ToLower(name), "hindsight") {
			continue
		}
		from, to := filepath.Join(src, name), filepath.Join(dest, name)
		info, err := os.Stat(from) // follows links
		if err != nil {
			continue // dangling link
		}
		switch {
		case info.IsDir():
			if err := copyTree(from, to, depth+1, false, top && name == "extensions"); err != nil {
				return err
			}
		case info.Mode().IsRegular():
			if err := copyFile(from, to, info.Mode().Perm()); err != nil {
				return err
			}
		}
	}
	return nil
}

func copyFile(from, to string, perm os.FileMode) error {
	in, err := os.Open(from)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	out, err := os.OpenFile(to, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

// SetupHash is a short fingerprint of what steers the agent in a copied Pi
// folder (settings, AGENTS.md, skills, extensions), so a changed setup shows
// when personal results are compared later.
func SetupHash(dir string) string {
	h := sha256.New()
	var files []string
	for _, f := range []string{"settings.json", "AGENTS.md"} {
		files = append(files, filepath.Join(dir, f))
	}
	for _, sub := range []string{"skills", "extensions"} {
		_ = filepath.WalkDir(filepath.Join(dir, sub), func(p string, d fs.DirEntry, err error) error {
			if err == nil && d.Type().IsRegular() {
				files = append(files, p)
			}
			return nil
		})
	}
	sort.Strings(files)
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		rel, _ := filepath.Rel(dir, f)
		_, _ = fmt.Fprintf(h, "%s\x00%s\x00", rel, data)
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// preparePersonalAgentDir builds the Pi folder of a personal run: a copy of
// the user's own, with the pill provider pointed at the model under test.
func preparePersonalAgentDir(src, dir string, port int, m pi.Model) (string, error) {
	if err := CopyPersonalPi(src, dir); err != nil {
		return "", err
	}
	m.Unverified = false
	if _, err := pi.Merge(pi.ModelsPath(dir), pi.BuildProvider(port, []pi.Model{m})); err != nil {
		return "", err
	}
	return SetupHash(dir), nil
}
