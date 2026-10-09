package config

import (
	"bytes"
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

// defaultLine matches the top-level `default` key, whatever its indentation
// or quoting ( default = ..., "default" = ..., 'default' = ... ).
var defaultLine = regexp.MustCompile(`^[ \t]*(?:default|"default"|'default')[ \t]*=.*$`)

// topLevelDefaults walks config.toml line by line and returns the byte ranges
// of the `default = ...` lines that sit at the top level, before the first
// [table] or [[array]] header.
//
// A plain regexp is not enough: a "[" at the start of a line can continue a
// multi-line array ("extra = [\n  [1, 2]\n]"), and "default" can appear inside
// a multi-line string. So the scan tracks bracket depth, strings and comments,
// and only looks at lines that start at depth zero outside any string.
func topLevelDefaults(data []byte) (defaults [][2]int) {
	depth := 0
	multi := "" // the closing delimiter while inside a """ or ''' string
	open := -1  // index in defaults of a value that has not ended yet
	pos := 0
	for pos < len(data) {
		next := len(data)
		if nl := bytes.IndexByte(data[pos:], '\n'); nl >= 0 {
			next = pos + nl + 1
		}
		line := string(bytes.TrimRight(data[pos:next], "\r\n"))
		if depth == 0 && multi == "" {
			if strings.HasPrefix(strings.TrimLeft(line, " \t"), "[") {
				return defaults
			}
			if defaultLine.MatchString(line) {
				defaults = append(defaults, [2]int{pos, pos + len(line)})
				open = len(defaults) - 1
			}
		}
		depth, multi = scanLine(line, depth, multi)
		// A value can run over several lines (default = """\nname"""): the
		// replaced range must reach the line where it ends.
		if open >= 0 {
			defaults[open][1] = pos + len(line)
			if depth == 0 && multi == "" {
				open = -1
			}
		}
		pos = next
	}
	return defaults
}

// scanLine updates the bracket depth and multi-line string state after one line.
func scanLine(line string, depth int, multi string) (int, string) {
	for i := 0; i < len(line); i++ {
		if multi != "" {
			if strings.HasPrefix(line[i:], multi) {
				// TOML lets a multi-line string end with up to two extra quote
				// characters before the delimiter ("""text"""" is text").
				run := 0
				for i+run < len(line) && line[i+run] == multi[0] {
					run++
				}
				i += run - 1
				multi = ""
			} else if multi == `"""` && line[i] == '\\' {
				i++ // an escaped character, possibly a quote
			}
			continue
		}
		switch c := line[i]; c {
		case '#':
			return depth, multi // a comment runs to the end of the line
		case '[', '{':
			depth++
		case ']', '}':
			if depth > 0 {
				depth--
			}
		case '"', '\'':
			if q := strings.Repeat(string(c), 3); strings.HasPrefix(line[i:], q) {
				multi = q
				i += 2
				continue
			}
			for i++; i < len(line) && line[i] != c; i++ {
				if c == '"' && line[i] == '\\' {
					i++
				}
			}
		}
	}
	return depth, multi
}

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
	default:
		// Only the top level counts: a "default" key inside a [table] is
		// somebody else's and must stay untouched.
		if defaults := topLevelDefaults(data); len(defaults) > 0 {
			var out []byte
			prev := 0
			for _, r := range defaults {
				out = append(out, data[prev:r[0]]...)
				out = append(out, line...)
				prev = r[1]
			}
			data = append(out, data[prev:]...)
		} else {
			// toml requires top-level keys before any [table]; put it first.
			data = append([]byte(line+"\n"), data...)
		}
	}
	// Never write a file pill itself could not read back.
	var check File
	if _, err := toml.Decode(string(data), &check); err != nil || check.Default != name {
		return fmt.Errorf("cannot set the default in %s without breaking it; edit the file by hand (%v)", p.ConfigFile(), err)
	}
	return WriteFileAtomic(p.ConfigFile(), data, 0o644)
}
