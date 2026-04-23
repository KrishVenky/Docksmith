package util

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

// HashBytes returns "sha256:<hex>" for the given byte slice.
func HashBytes(data []byte) string {
	h := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(h[:])
}

// HashFile returns "sha256:<hex>" for the contents of the file at path.
func HashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("hash file: %w", err)
	}
	defer f.Close()
	return HashReader(f)
}

// HashReader streams through r and returns "sha256:<hex>".
func HashReader(r io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", fmt.Errorf("hash reader: %w", err)
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}
