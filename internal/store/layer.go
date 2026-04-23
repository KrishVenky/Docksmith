package store

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"docksmith/internal/util"
)

// WriteLayer writes tar data to layers/ named by its SHA-256 digest.
// Returns the digest string and byte size.
func (s *Store) WriteLayer(tarData []byte) (string, int64, error) {
	digest := util.HashBytes(tarData)
	// digest is "sha256:abc123...", filename uses the full string with : replaced
	fileName := strings.ReplaceAll(digest, ":", "-")
	path := filepath.Join(s.LayersDir(), fileName)

	if err := os.WriteFile(path, tarData, 0644); err != nil {
		return "", 0, fmt.Errorf("write layer: %w", err)
	}
	s.fixOwnership(path)
	return digest, int64(len(tarData)), nil
}

// ReadLayer reads a layer's tar data by its digest.
func (s *Store) ReadLayer(digest string) ([]byte, error) {
	path := s.LayerPath(digest)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read layer %s: %w", digest, err)
	}
	return data, nil
}

// LayerPath returns the filesystem path for a given layer digest.
func (s *Store) LayerPath(digest string) string {
	fileName := strings.ReplaceAll(digest, ":", "-")
	return filepath.Join(s.LayersDir(), fileName)
}

// LayerExists checks if a layer with the given digest exists on disk.
func (s *Store) LayerExists(digest string) bool {
	_, err := os.Stat(s.LayerPath(digest))
	return err == nil
}

// RemoveLayer deletes the layer file for the given digest.
func (s *Store) RemoveLayer(digest string) error {
	path := s.LayerPath(digest)
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("layer %s not found", digest)
		}
		return fmt.Errorf("remove layer: %w", err)
	}
	return nil
}
