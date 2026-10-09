package hf

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseRef(t *testing.T) {
	cases := []struct {
		in      string
		want    Ref
		wantErr bool
	}{
		{"hf.co/unsloth/gemma-GGUF:Q4_K_M", Ref{Repo: "unsloth/gemma-GGUF", Quant: "Q4_K_M"}, false},
		{"https://huggingface.co/o/r:IQ4_XS", Ref{Repo: "o/r", Quant: "IQ4_XS"}, false},
		{"o/r/model-Q8_0.gguf", Ref{Repo: "o/r", File: "model-Q8_0.gguf"}, false},
		{"hf.co/o/r/sub/dir/m.gguf", Ref{Repo: "o/r", File: "sub/dir/m.gguf"}, false},
		{"o/r", Ref{}, true},
		{"o/r:", Ref{}, true},
		{"o/r/readme.md", Ref{}, true},
		{"justaname", Ref{}, true},
	}
	for _, c := range cases {
		got, err := ParseRef(c.in)
		if (err != nil) != c.wantErr || got != c.want {
			t.Errorf("ParseRef(%q) = %+v, %v", c.in, got, err)
		}
	}
	if LooksLikeRef("gemma4-26b") || LooksLikeRef("x.gguf") || !LooksLikeRef("o/r:Q4") || !LooksLikeRef("hf.co/o/r:Q4") {
		t.Error("LooksLikeRef misclassified an input")
	}
}

func TestBaseName(t *testing.T) {
	if got := (Ref{Repo: "unsloth/Qwen3-4B-GGUF", Quant: "Q4_K_M"}).BaseName(); got != "qwen3-4b" {
		t.Errorf("got %s", got)
	}
	if got := (Ref{Repo: "o/r", File: "sub/My_Model-Q8_0.gguf"}).BaseName(); got != "my-model-q8-0" {
		t.Errorf("got %s", got)
	}
	if ShortQuant("Q4_K_M") != "q4km" || ShortQuant("UD-IQ4_XS") != "iq4xs" {
		t.Error("ShortQuant wrong")
	}
}

func TestFindFile(t *testing.T) {
	files := []File{
		{Path: "README.md"},
		{Path: "m-Q4_K_M.gguf"}, {Path: "m-Q4_K_S.gguf"}, {Path: "m-Q8_0.gguf"},
		{Path: "mmproj-F16.gguf"}, {Path: "imatrix.gguf"},
		{Path: "big-Q6_K-00001-of-00002.gguf"}, {Path: "big-Q6_K-00002-of-00002.gguf"},
		{Path: "UD-IQ4_XS/x.gguf"}, {Path: "a-Q5_K.gguf"}, {Path: "a-Q5_K_M.gguf"}, {Path: "b-Q2_K.gguf"}, {Path: "b-UD-Q2_K.gguf"},
	}
	cases := []struct {
		ref     Ref
		want    string
		wantErr string
	}{
		{Ref{Quant: "Q4_K_M"}, "m-Q4_K_M.gguf", ""},
		{Ref{Quant: "q8_0"}, "m-Q8_0.gguf", ""},
		{Ref{Quant: "F16"}, "", "no GGUF"}, // mmproj is not a model
		{Ref{Quant: "Q6_K"}, "", "split"},
		{Ref{Quant: "Q3_K_L"}, "", "no GGUF"},
		{Ref{Quant: "Q5_K"}, "a-Q5_K.gguf", ""}, // does not match Q5_K_M
		{Ref{Quant: "Q2_K"}, "", "matches several"},
		{Ref{File: "m-Q8_0.gguf"}, "m-Q8_0.gguf", ""},
		{Ref{File: "nope.gguf"}, "", "no file"},
	}
	for _, c := range cases {
		c.ref.Repo = "o/r"
		f, err := FindFile(files, c.ref)
		if c.wantErr != "" {
			if err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Errorf("%+v: err = %v, want %q", c.ref, err, c.wantErr)
			}
			continue
		}
		if err != nil || f.Path != c.want {
			t.Errorf("%+v: got %v, %v; want %s", c.ref, f, err, c.want)
		}
	}
}

// hubServer fakes Hugging Face: a tree API and resolve URLs with Range support.
type hubServer struct {
	*httptest.Server
	content     []byte
	oid         string
	ignoreRange bool
	gotAuth     string
	rangeSeen   string
	treeStatus  int
}

func newHub(t *testing.T, content []byte) *hubServer {
	t.Helper()
	sum := sha256.Sum256(content)
	hs := &hubServer{content: content, oid: hex.EncodeToString(sum[:]), treeStatus: 200}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/models/o/r/tree/main", func(w http.ResponseWriter, r *http.Request) {
		hs.gotAuth = r.Header.Get("Authorization")
		if hs.treeStatus != 200 {
			w.WriteHeader(hs.treeStatus)
			return
		}
		if r.URL.Query().Get("page") == "" {
			// First page: one entry plus a Link header to page two.
			w.Header().Set("Link", fmt.Sprintf(`<%s/api/models/o/r/tree/main?page=2>; rel="next"`, hs.URL))
			_ = json.NewEncoder(w).Encode([]map[string]any{{"type": "directory", "path": "dir"}, {"type": "file", "path": "README.md", "size": 3}})
			return
		}
		_ = json.NewEncoder(w).Encode([]map[string]any{{
			"type": "file", "path": "m-Q4_K_M.gguf", "size": 1,
			"lfs": map[string]any{"oid": hs.oid, "size": len(content)},
		}})
	})
	mux.HandleFunc("/o/r/resolve/main/", func(w http.ResponseWriter, r *http.Request) {
		hs.gotAuth = r.Header.Get("Authorization")
		hs.rangeSeen = r.Header.Get("Range")
		if hs.ignoreRange {
			r.Header.Del("Range")
		}
		http.ServeContent(w, r, "m.gguf", time.Time{}, bytes.NewReader(hs.content))
	})
	hs.Server = httptest.NewServer(mux)
	t.Cleanup(hs.Close)
	return hs
}

func (hs *hubServer) client() *Client {
	return &Client{Endpoint: hs.URL, HTTP: hs.Client()}
}

func payload() []byte { return bytes.Repeat([]byte("0123456789abcdef"), 4096) } // 64 KiB

func TestListFilesFollowsPaginationAndReadsLFS(t *testing.T) {
	hs := newHub(t, payload())
	c := hs.client()
	c.Token = "tok"
	files, err := c.ListFiles(context.Background(), "o/r")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 || files[1].Path != "m-Q4_K_M.gguf" || files[1].SHA256 != hs.oid || files[1].Size != int64(len(payload())) {
		t.Errorf("files = %+v", files)
	}
	if hs.gotAuth != "Bearer tok" {
		t.Errorf("HF_TOKEN was not sent: %q", hs.gotAuth)
	}
}

func TestListFilesAuthError(t *testing.T) {
	hs := newHub(t, payload())
	hs.treeStatus = 401
	_, err := hs.client().ListFiles(context.Background(), "o/r")
	if _, ok := err.(*AuthError); !ok {
		t.Errorf("err = %T %v", err, err)
	}
}

func lfsFile(hs *hubServer) File {
	return File{Path: "m-Q4_K_M.gguf", Size: int64(len(hs.content)), SHA256: hs.oid}
}

func TestDownloadFresh(t *testing.T) {
	hs := newHub(t, payload())
	dir := t.TempDir()
	var lastDone int64
	sum, err := hs.client().Download(context.Background(), "o/r", lfsFile(hs), dir, func(done, total int64) { lastDone = done })
	if err != nil || sum != hs.oid {
		t.Fatalf("sum=%s err=%v", sum, err)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "m-Q4_K_M.gguf"))
	if !bytes.Equal(got, payload()) {
		t.Error("content differs")
	}
	if _, err := os.Stat(filepath.Join(dir, "m-Q4_K_M.gguf.partial")); err == nil {
		t.Error(".partial must be gone after success")
	}
	if lastDone != int64(len(payload())) {
		t.Errorf("progress ended at %d", lastDone)
	}
}

func TestDownloadResumesPartialFile(t *testing.T) {
	hs := newHub(t, payload())
	dir := t.TempDir()
	half := len(payload()) / 2
	if err := os.WriteFile(filepath.Join(dir, "m-Q4_K_M.gguf.partial"), payload()[:half], 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := hs.client().Download(context.Background(), "o/r", lfsFile(hs), dir, nil); err != nil {
		t.Fatal(err)
	}
	if want := fmt.Sprintf("bytes=%d-", half); hs.rangeSeen != want {
		t.Errorf("Range = %q, want %q", hs.rangeSeen, want)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "m-Q4_K_M.gguf"))
	if !bytes.Equal(got, payload()) {
		t.Error("resumed content differs (sha256 over the whole file was still verified)")
	}
}

func TestDownloadRestartsWhenServerIgnoresRange(t *testing.T) {
	hs := newHub(t, payload())
	hs.ignoreRange = true
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "m-Q4_K_M.gguf.partial"), []byte("garbage-prefix"), 0o644)
	if _, err := hs.client().Download(context.Background(), "o/r", lfsFile(hs), dir, nil); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "m-Q4_K_M.gguf"))
	if !bytes.Equal(got, payload()) {
		t.Error("content differs after restart")
	}
}

func TestDownloadRejectsBadChecksumAndKeepsNothing(t *testing.T) {
	hs := newHub(t, payload())
	f := lfsFile(hs)
	f.SHA256 = strings.Repeat("0", 64)
	dir := t.TempDir()
	_, err := hs.client().Download(context.Background(), "o/r", f, dir, nil)
	if !IsChecksumError(err) {
		t.Fatalf("err = %v", err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Errorf("corrupt download left files behind: %v", entries)
	}
}

func TestDownloadAlreadyCompletePartial(t *testing.T) {
	hs := newHub(t, payload())
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "m-Q4_K_M.gguf.partial"), payload(), 0o644)
	if _, err := hs.client().Download(context.Background(), "o/r", lfsFile(hs), dir, nil); err != nil {
		t.Fatal(err)
	}
	if hs.rangeSeen != "" {
		t.Error("a complete partial file needs no request")
	}
}
