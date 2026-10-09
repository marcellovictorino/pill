// Package hf talks to Hugging Face: parsing model references and downloading
// GGUF files.
package hf

import (
	"fmt"
	"regexp"
	"strings"
)

// Ref points at a GGUF on Hugging Face, either by quantisation
// ("owner/repo:Q4_K_M") or by exact file ("owner/repo/file.gguf").
type Ref struct {
	Repo  string // owner/name
	Quant string // set for the :quant form
	File  string // set for the file form
}

var refPrefixes = []string{"https://huggingface.co/", "http://huggingface.co/", "huggingface.co/", "hf.co/", "hf:"}

func stripPrefix(s string) string {
	for _, p := range refPrefixes {
		if strings.HasPrefix(s, p) {
			return strings.TrimPrefix(s, p)
		}
	}
	return s
}

// LooksLikeRef reports whether s should be parsed as a Hugging Face reference
// rather than a pill model name or a local file name. Pill names and bare
// file names never contain a slash.
func LooksLikeRef(s string) bool { return strings.Contains(stripPrefix(s), "/") }

// ParseRef parses "hf.co/owner/repo:QUANT" and "owner/repo/path/file.gguf"
// (any of the usual huggingface.co prefixes is accepted and ignored).
func ParseRef(s string) (Ref, error) {
	rest := stripPrefix(strings.TrimSpace(s))
	parts := strings.Split(rest, "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return Ref{}, fmt.Errorf("%q is not a Hugging Face reference; use owner/repo:QUANT or owner/repo/file.gguf", s)
	}
	if len(parts) == 2 {
		repoName, quant, ok := strings.Cut(parts[1], ":")
		if !ok || quant == "" {
			return Ref{}, fmt.Errorf("%q needs a quantisation (owner/repo:Q4_K_M) or a file (owner/repo/file.gguf)", s)
		}
		return Ref{Repo: parts[0] + "/" + repoName, Quant: quant}, nil
	}
	file := strings.Join(parts[2:], "/")
	if !strings.HasSuffix(strings.ToLower(file), ".gguf") {
		return Ref{}, fmt.Errorf("%q: %q is not a .gguf file", s, file)
	}
	return Ref{Repo: parts[0] + "/" + parts[1], File: file}, nil
}

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

func slug(s string) string {
	return strings.Trim(nonAlnum.ReplaceAllString(strings.ToLower(s), "-"), "-")
}

// BaseName derives the stem of a pill name from the reference: the repo name
// without a trailing "-GGUF" (quant form) or the file stem (file form).
func (r Ref) BaseName() string {
	if r.File != "" {
		stem := r.File[strings.LastIndex(r.File, "/")+1:]
		return slug(strings.TrimSuffix(stem, stem[strings.LastIndex(stem, "."):]))
	}
	name := r.Repo[strings.Index(r.Repo, "/")+1:]
	name = strings.TrimSuffix(strings.TrimSuffix(name, "-GGUF"), "-gguf")
	return slug(name)
}

// ShortQuant lower-cases a quantisation and drops separators: Q4_K_M -> q4km.
func ShortQuant(q string) string {
	q = strings.TrimPrefix(q, "UD-")
	return strings.NewReplacer("_", "", "-", "").Replace(strings.ToLower(q))
}
