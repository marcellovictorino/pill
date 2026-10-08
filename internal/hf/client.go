package hf

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"strings"
	"time"
)

// DefaultEndpoint is the Hugging Face site; PILL_HF_ENDPOINT overrides it (tests).
const DefaultEndpoint = "https://huggingface.co"

// Client downloads from Hugging Face. The token comes from HF_TOKEN in the
// environment and is never stored by pill.
type Client struct {
	Endpoint string
	Token    string
	HTTP     *http.Client
}

// NewClient builds a client from the environment.
func NewClient() *Client {
	endpoint := os.Getenv("PILL_HF_ENDPOINT")
	if endpoint == "" {
		endpoint = DefaultEndpoint
	}
	return &Client{
		Endpoint: strings.TrimRight(endpoint, "/"),
		Token:    os.Getenv("HF_TOKEN"),
		// No overall Timeout: a 13 GB download legitimately takes many minutes.
		// Connection setup and response headers are still bounded by the transport.
		HTTP: &http.Client{Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			TLSHandshakeTimeout:   20 * time.Second,
			ResponseHeaderTimeout: 60 * time.Second,
		}},
	}
}

// File is one file in a repo listing.
type File struct {
	Path   string
	Size   int64
	SHA256 string // from Git LFS metadata; empty for non-LFS files
}

// Name is the file's base name (the tree can list files inside folders).
func (f File) Name() string { return path.Base(f.Path) }

// AuthError is returned for HTTP 401/403/404 so callers can hint at HF_TOKEN.
type AuthError struct {
	Status int
	URL    string
}

func (e *AuthError) Error() string {
	return fmt.Sprintf("Hugging Face returned HTTP %d for %s", e.Status, e.URL)
}

func (c *Client) newRequest(ctx context.Context, method, u string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, u, nil)
	if err != nil {
		return nil, err
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	req.Header.Set("User-Agent", "pill")
	return req, nil
}

var nextLink = regexp.MustCompile(`<([^>]+)>;\s*rel="next"`)

// ListFiles lists every file in the repo's main branch, following pagination.
func (c *Client) ListFiles(ctx context.Context, repo string) ([]File, error) {
	next := fmt.Sprintf("%s/api/models/%s/tree/main?recursive=true", c.Endpoint, repo)
	var files []File
	for page := 0; next != "" && page < 50; page++ {
		req, err := c.newRequest(ctx, http.MethodGet, next)
		if err != nil {
			return nil, err
		}
		resp, err := c.HTTP.Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			if resp.StatusCode == 401 || resp.StatusCode == 403 || resp.StatusCode == 404 {
				return nil, &AuthError{Status: resp.StatusCode, URL: next}
			}
			return nil, fmt.Errorf("list %s: HTTP %d", repo, resp.StatusCode)
		}
		var entries []struct {
			Type string `json:"type"`
			Path string `json:"path"`
			Size int64  `json:"size"`
			LFS  *struct {
				OID  string `json:"oid"`
				Size int64  `json:"size"`
			} `json:"lfs"`
		}
		err = json.NewDecoder(resp.Body).Decode(&entries)
		link := resp.Header.Get("Link")
		_ = resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("list %s: %w", repo, err)
		}
		for _, e := range entries {
			if e.Type != "file" {
				continue
			}
			f := File{Path: e.Path, Size: e.Size}
			if e.LFS != nil {
				f.SHA256, f.Size = e.LFS.OID, e.LFS.Size
			}
			files = append(files, f)
		}
		next = ""
		if m := nextLink.FindStringSubmatch(link); m != nil {
			next = m[1]
			if !strings.HasPrefix(next, "http") {
				next = c.Endpoint + next
			}
		}
	}
	return files, nil
}

var splitShard = regexp.MustCompile(`-\d{5}-of-\d{5}\.gguf$`)

// FindFile picks the GGUF a reference points at: the exact file for the
// file form, or the single non-split GGUF matching the quantisation.
func FindFile(files []File, ref Ref) (File, error) {
	if ref.File != "" {
		// A path (sub/model.gguf) must match exactly. A bare name matches by
		// base name, and when several folders hold that name the caller has to
		// say which one, instead of getting whichever came first.
		var byName []File
		for _, f := range files {
			if f.Path == ref.File {
				return f, nil
			}
			if !strings.Contains(ref.File, "/") && f.Name() == ref.File {
				byName = append(byName, f)
			}
		}
		switch len(byName) {
		case 1:
			return byName[0], nil
		case 0:
			return File{}, fmt.Errorf("%s has no file %q", ref.Repo, ref.File)
		}
		paths := make([]string, len(byName))
		for i, f := range byName {
			paths[i] = f.Path
		}
		return File{}, fmt.Errorf("%s has several files named %q (%s); name one with its folder: pill pull %s/<path>", ref.Repo, ref.File, strings.Join(paths, ", "), ref.Repo)
	}
	q := strings.ToLower(ref.Quant)
	boundary := regexp.MustCompile(`(^|[-._])` + regexp.QuoteMeta(q) + `(\.gguf$|[-.])`)
	var hits []File
	splits := 0
	for _, f := range files {
		name := strings.ToLower(f.Name())
		if !strings.HasSuffix(name, ".gguf") || strings.HasPrefix(name, "mmproj") || strings.Contains(name, "imatrix") {
			continue
		}
		if !boundary.MatchString(name) {
			continue
		}
		if splitShard.MatchString(name) {
			splits++
			continue
		}
		hits = append(hits, f)
	}
	switch {
	case len(hits) == 1:
		return hits[0], nil
	case len(hits) > 1:
		// Prefer the file that ends in "-<quant>.gguf" exactly.
		var exact []File
		for _, f := range hits {
			if strings.HasSuffix(strings.ToLower(f.Name()), "-"+q+".gguf") {
				exact = append(exact, f)
			}
		}
		if len(exact) == 1 {
			return exact[0], nil
		}
		names := make([]string, len(hits))
		for i, f := range hits {
			names[i] = f.Name()
		}
		return File{}, fmt.Errorf("%s matches several files (%s); name one exactly with owner/repo/file.gguf", ref.Quant, strings.Join(names, ", "))
	case splits > 0:
		return File{}, fmt.Errorf("%s is split into several GGUF shards, which pill does not support yet", ref.Quant)
	}
	return File{}, fmt.Errorf("%s has no GGUF for quantisation %q", ref.Repo, ref.Quant)
}

// errChecksum marks a downloaded file whose sha256 differs from Hugging Face's.
var errChecksum = errors.New("sha256 mismatch")

// IsChecksumError reports whether err came from a failed sha256 check.
func IsChecksumError(err error) bool { return errors.Is(err, errChecksum) }

func (c *Client) resolveURL(repo, file string) string {
	parts := strings.Split(file, "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	return fmt.Sprintf("%s/%s/resolve/main/%s", c.Endpoint, repo, strings.Join(parts, "/"))
}
