// Package config resolves pill's directories and reads and writes its small
// files: config.toml, models.toml and the machine-local state in results.json.
package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// Paths are the two roots pill uses (see AGENTS.md for the layout):
//
//	ConfigDir  ~/.config/pill  portable and small, safe to keep in dotfiles
//	Root       ~/.pill         machine-local: GGUFs, generated files, logs
type Paths struct {
	ConfigDir string
	Root      string
}

// Resolve works out the roots. PILL_HOME relocates everything under one
// directory (config in PILL_HOME/config); tests use it so they never touch
// the real home directory.
func Resolve() (Paths, error) {
	if h := os.Getenv("PILL_HOME"); h != "" {
		return Paths{ConfigDir: filepath.Join(h, "config"), Root: h}, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, fmt.Errorf("cannot find home directory: %w", err)
	}
	cfg := os.Getenv("XDG_CONFIG_HOME")
	if cfg == "" {
		cfg = filepath.Join(home, ".config")
	}
	return Paths{ConfigDir: filepath.Join(cfg, "pill"), Root: filepath.Join(home, ".pill")}, nil
}

func (p Paths) ConfigFile() string { return filepath.Join(p.ConfigDir, "config.toml") }
func (p Paths) ModelsToml() string { return filepath.Join(p.ConfigDir, "models.toml") }
func (p Paths) DefaultModelsDir() string {
	return filepath.Join(p.Root, "models")
}
func (p Paths) ModelsIni() string    { return filepath.Join(p.Root, "models.ini") }
func (p Paths) ResultsFile() string  { return filepath.Join(p.Root, "results.json") }
func (p Paths) LogsDir() string      { return filepath.Join(p.Root, "logs") }
func (p Paths) ServerLog() string    { return filepath.Join(p.LogsDir(), "server.log") }
func (p Paths) RunDir() string       { return filepath.Join(p.Root, "run") }
func (p Paths) PidFile() string      { return filepath.Join(p.RunDir(), "server.pid") }
func (p Paths) RouterState() string  { return filepath.Join(p.RunDir(), "router.json") }
func (p Paths) BenchmarkDir() string { return filepath.Join(p.Root, "benchmark") }
