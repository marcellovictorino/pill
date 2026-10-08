package testutil

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// Hub is a fake Hugging Face server: a tree API and resolve URLs with Range
// support (http.ServeContent handles Range for us). Repos map "owner/name"
// to file path to content.
type Hub struct {
	*httptest.Server
	Repos map[string]map[string][]byte

	mu       sync.Mutex
	Requests []string // "METHOD path" log
	Status   int      // when non-zero, every request fails with this status
	LastAuth string
}

// NewHub starts the fake and points PILL_HF_ENDPOINT at it.
func NewHub(t *testing.T, repos map[string]map[string][]byte) *Hub {
	t.Helper()
	h := &Hub{Repos: repos}
	h.Server = httptest.NewServer(http.HandlerFunc(h.serve))
	t.Cleanup(h.Close)
	t.Setenv("PILL_HF_ENDPOINT", h.URL)
	return h
}

// Hits counts logged requests whose line contains substr.
func (h *Hub) Hits(substr string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	n := 0
	for _, r := range h.Requests {
		if strings.Contains(r, substr) {
			n++
		}
	}
	return n
}

func (h *Hub) serve(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	h.Requests = append(h.Requests, r.Method+" "+r.URL.Path)
	h.LastAuth = r.Header.Get("Authorization")
	status := h.Status
	h.mu.Unlock()
	if status != 0 {
		w.WriteHeader(status)
		return
	}

	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	switch {
	case len(parts) >= 6 && parts[0] == "api" && parts[1] == "models" && parts[4] == "tree":
		repo := parts[2] + "/" + parts[3]
		files, ok := h.Repos[repo]
		if !ok {
			http.NotFound(w, r)
			return
		}
		var entries []map[string]any
		for path, content := range files {
			sum := sha256.Sum256(content)
			entries = append(entries, map[string]any{
				"type": "file", "path": path, "size": len(content),
				"lfs": map[string]any{"oid": hex.EncodeToString(sum[:]), "size": len(content)},
			})
		}
		_ = json.NewEncoder(w).Encode(entries)
	case len(parts) >= 5 && parts[2] == "resolve":
		repo := parts[0] + "/" + parts[1]
		path := strings.Join(parts[4:], "/")
		content, ok := h.Repos[repo][path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		http.ServeContent(w, r, path, time.Time{}, bytes.NewReader(content))
	default:
		http.NotFound(w, r)
	}
}
