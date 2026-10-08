package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

// Defaults for settings that config.toml and the environment can override.
const (
	DefaultPort        = 11435
	DefaultIdleSeconds = 900
)

// File is the on-disk shape of config.toml. The `toml:"..."` struct tags tell
// the TOML library which key maps to which field.
type File struct {
	Port        int    `toml:"port"`
	IdleSeconds int    `toml:"idle_seconds"`
	Default     string `toml:"default"`
	ModelsDir   string `toml:"models_dir"`
}

// Settings are the effective values: file contents, then defaults, then
// environment overrides (PILL_PORT).
type Settings struct {
	Port        int
	IdleSeconds int
	Default     string
	ModelsDir   string
}

// Load reads config.toml (a missing file is fine) and applies defaults and
// the PILL_PORT override.
func Load(p Paths) (Settings, error) {
	var f File
	if _, err := toml.DecodeFile(p.ConfigFile(), &f); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return Settings{}, fmt.Errorf("read %s: %w", p.ConfigFile(), err)
	}
	s := Settings(f)
	if s.Port == 0 {
		s.Port = DefaultPort
	}
	if env := os.Getenv("PILL_PORT"); env != "" {
		port, err := strconv.Atoi(env)
		if err != nil || port < 1 || port > 65535 {
			return Settings{}, fmt.Errorf("PILL_PORT=%q is not a valid port", env)
		}
		s.Port = port
	}
	if s.IdleSeconds == 0 {
		s.IdleSeconds = DefaultIdleSeconds
	}
	if s.ModelsDir == "" {
		s.ModelsDir = p.DefaultModelsDir()
	} else if strings.HasPrefix(s.ModelsDir, "~/") {
		home, _ := os.UserHomeDir()
		s.ModelsDir = filepath.Join(home, s.ModelsDir[2:])
	}
	return s, nil
}

var defaultLine = regexp.MustCompile(`(?m)^default\s*=.*$`)

const configHeader = "# pill settings. Optional keys: port = 11435, idle_seconds = 900, models_dir = \"~/.pill/models\"\n"

// SetDefault records the default model in config.toml. It edits only the
// `default` line, so comments and other keys a person added (this file is
// meant to live in dotfiles) survive.
func SetDefault(p Paths, name string) error {
	line := "default = " + strconv.Quote(name)
	data, err := os.ReadFile(p.ConfigFile())
	switch {
	case errors.Is(err, fs.ErrNotExist):
		data = []byte(configHeader + line + "\n")
	case err != nil:
		return err
	case defaultLine.Match(data):
		data = defaultLine.ReplaceAll(data, []byte(line))
	default:
		// toml requires top-level keys before any [table]; prepend to be safe.
		data = append([]byte(line+"\n"), data...)
	}
	return WriteFileAtomic(p.ConfigFile(), data, 0o644)
}
