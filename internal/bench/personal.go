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
		if errors.Is(err, fs.ErrNotExist) && isLink(from) {
			continue // a dangling link: nothing to copy
		} else if err != nil {
			return err // an unreadable entry must not silently shrink the setup
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

func isLink(p string) bool {
	fi, err := os.Lstat(p)
	return err == nil && fi.Mode()&os.ModeSymlink != 0
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
	for _, f := range []string{"settings.json", "AGENTS.md", "npm/package.json", "npm/package-lock.json"} {
		files = append(files, filepath.Join(dir, f))
	}
	walk := func(root string) {
		_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() && d.Name() == "node_modules" {
				return filepath.SkipDir
			}
			if d.Type().IsRegular() {
				files = append(files, p)
			}
			return nil
		})
	}
	for _, sub := range []string{"skills", "extensions"} {
		walk(filepath.Join(dir, sub))
	}
	// Settings can name extensions, skills and packages that live outside the
	// folder: their contents steer the agent too.
	if data, err := os.ReadFile(filepath.Join(dir, "settings.json")); err == nil {
		var cfg map[string]any
		if json.Unmarshal(data, &cfg) == nil {
			for _, key := range []string{"extensions", "skills", "packages"} {
				list, _ := cfg[key].([]any)
				for _, e := range list {
					if path, ok := e.(string); ok {
						path = resolveRef(dir, path)
						walk(path)
						files = append(files, path)
					}
				}
			}
		}
	}
	sort.Strings(files)
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue // a directory, or a name that is not a path
		}
		name := f
		if rel, err := filepath.Rel(dir, f); err == nil && !strings.HasPrefix(rel, "..") {
			name = rel
		}
		_, _ = fmt.Fprintf(h, "%s\x00%s\x00", name, data)
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// resolveRef turns a path named in settings.json into a real one: ~/ is the
// home folder and a relative path is relative to the Pi folder, as for Pi
// itself. A package identifier resolves to a path that does not exist and so
// contributes nothing.
func resolveRef(dir, p string) string {
	switch {
	case strings.HasPrefix(p, "~/"):
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, p[2:])
		}
	case !filepath.IsAbs(p):
		return filepath.Join(dir, p)
	}
	return p
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
