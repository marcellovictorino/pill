// Package store keeps the notes in a JSON file under $NOTES_HOME.
package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Note is one stored note.
type Note struct {
	ID      int       `json:"id"`
	Text    string    `json:"text"`
	Tags    []string  `json:"tags"`
	Created time.Time `json:"created"`
}

type file struct {
	NextID int    `json:"next_id"`
	Notes  []Note `json:"notes"`
}

// Store is an opened notes file.
type Store struct {
	path string
	data file
}

// Dir returns the data directory: $NOTES_HOME, else ~/.notes.
func Dir() (string, error) {
	if d := os.Getenv("NOTES_HOME"); d != "" {
		return d, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".notes"), nil
}

// Now is the clock for new notes. NOTES_NOW (RFC 3339) overrides it so tests
// can control timestamps.
func Now() time.Time {
	if v := os.Getenv("NOTES_NOW"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			return t.UTC()
		}
	}
	return time.Now().UTC().Truncate(time.Second)
}

// Open loads the notes file; a missing file is an empty store.
func Open() (*Store, error) {
	dir, err := Dir()
	if err != nil {
		return nil, err
	}
	s := &Store{path: filepath.Join(dir, "notes.json"), data: file{NextID: 1}}
	b, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &s.data); err != nil {
		return nil, fmt.Errorf("read %s: %w", s.path, err)
	}
	if s.data.NextID < 1 {
		s.data.NextID = 1
	}
	return s, nil
}

// Notes returns the notes in id order.
func (s *Store) Notes() []Note {
	out := make([]Note, len(s.data.Notes))
	copy(out, s.data.Notes)
	return out
}

// Add stores a new note and returns it.
func (s *Store) Add(text string, tags []string) Note {
	n := Note{ID: s.data.NextID, Text: text, Tags: append([]string{}, tags...), Created: Now()}
	s.data.NextID++
	s.data.Notes = append(s.data.Notes, n)
	return n
}

// Save writes the file atomically (temp file, then rename).
func (s *Store) Save() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// HasTag reports whether the note carries the tag.
func (n Note) HasTag(tag string) bool {
	for _, t := range n.Tags {
		if strings.TrimSpace(t) == strings.TrimSpace(tag) {
			return true
		}
	}
	return false
}
