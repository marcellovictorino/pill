package hf

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Progress is called as bytes arrive; total is the full file size.
type Progress func(done, total int64)

// Download fetches file into dir under its base name and returns the
// sha256 of the result.
//
// Bytes go to "<name>.partial". If that file already exists the download
// resumes from its end with an HTTP Range request, so an interrupted 13 GB
// transfer does not start over. The sha256 is computed while writing and
// compared with Hugging Face's LFS checksum before the file is renamed into
// place, so a corrupt download never appears under its final name.
func (c *Client) Download(ctx context.Context, repo string, f File, dir string, progress Progress) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	final := filepath.Join(dir, f.Name())
	partial := final + ".partial"

	out, err := os.OpenFile(partial, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return "", err
	}
	defer func() { _ = out.Close() }()

	h := sha256.New()
	offset, err := out.Seek(0, io.SeekEnd)
	if err != nil {
		return "", err
	}
	if offset > f.Size && f.Size > 0 { // larger than the real file: junk, start over
		if err := out.Truncate(0); err != nil {
			return "", err
		}
		if _, err := out.Seek(0, io.SeekStart); err != nil {
			return "", err
		}
		offset = 0
	}
	if offset > 0 { // hash what we already have, so the final digest covers the whole file
		if _, err := out.Seek(0, io.SeekStart); err != nil {
			return "", err
		}
		if _, err := io.Copy(h, out); err != nil {
			return "", err
		}
	}

	if offset < f.Size || f.Size == 0 {
		offset, err = c.fetch(ctx, repo, f, out, h, offset, progress)
		if err != nil {
			return "", err
		}
	}
	if f.Size > 0 && offset != f.Size {
		return "", fmt.Errorf("download of %s ended at %d of %d bytes", f.Name(), offset, f.Size)
	}

	sum := hex.EncodeToString(h.Sum(nil))
	if f.SHA256 != "" && !strings.EqualFold(sum, f.SHA256) {
		_ = out.Close()
		_ = os.Remove(partial) // never resume from a corrupt file
		return sum, fmt.Errorf("%w for %s: got %s, Hugging Face says %s", errChecksum, f.Name(), sum, f.SHA256)
	}
	if err := out.Close(); err != nil {
		return sum, err
	}
	if err := os.Rename(partial, final); err != nil {
		return sum, err
	}
	return sum, nil
}

// fetch streams the file body into out, starting at offset.
func (c *Client) fetch(ctx context.Context, repo string, f File, out *os.File, h io.Writer, offset int64, progress Progress) (int64, error) {
	req, err := c.newRequest(ctx, http.MethodGet, c.resolveURL(repo, f.Path))
	if err != nil {
		return offset, err
	}
	if offset > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return offset, err
	}
	defer func() { _ = resp.Body.Close() }()

	switch resp.StatusCode {
	case http.StatusOK:
		if offset > 0 { // server ignored Range and sent everything: restart
			if _, err := out.Seek(0, io.SeekStart); err != nil {
				return offset, err
			}
			if err := out.Truncate(0); err != nil {
				return offset, err
			}
			offset = 0
			if hh, ok := h.(interface{ Reset() }); ok {
				hh.Reset()
			}
		}
	case http.StatusPartialContent:
	case http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound:
		return offset, &AuthError{Status: resp.StatusCode, URL: req.URL.String()}
	default:
		return offset, fmt.Errorf("download %s: HTTP %d", f.Name(), resp.StatusCode)
	}

	total := f.Size
	buf := make([]byte, 1<<20)
	last := time.Now()
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, err := out.Write(buf[:n]); err != nil {
				return offset, err
			}
			_, _ = h.Write(buf[:n])
			offset += int64(n)
			if progress != nil && time.Since(last) > 250*time.Millisecond {
				progress(offset, total)
				last = time.Now()
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return offset, rerr
		}
	}
	if progress != nil {
		progress(offset, total)
	}
	return offset, nil
}
