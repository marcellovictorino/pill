package hf

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
)

// HashFile returns the hex sha256 of a file, for checking that a file already
// on disk is the one Hugging Face lists.
func HashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
