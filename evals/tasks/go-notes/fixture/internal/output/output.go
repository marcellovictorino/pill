// Package output holds the output formats shared by every command.
package output

import (
	"encoding/json"
	"fmt"
	"io"
)

// Format is how a command prints its result.
type Format string

const (
	Human Format = "human"
	JSON  Format = "json"
)

// ParseFormat validates a --format value.
func ParseFormat(s string) (Format, error) {
	switch Format(s) {
	case Human, JSON:
		return Format(s), nil
	}
	return "", fmt.Errorf("unknown format %q (valid: human, json)", s)
}

// WriteJSON writes v as one line of compact JSON.
func WriteJSON(w io.Writer, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "%s\n", b)
	return err
}
