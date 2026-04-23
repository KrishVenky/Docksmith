package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"docksmith/internal/util"
)

// ImageConfig holds runtime configuration embedded in the image manifest.
type ImageConfig struct {
	Env        []string `json:"Env"`
	Cmd        []string `json:"Cmd"`
	WorkingDir string   `json:"WorkingDir"`
}

// LayerRef is a reference to a layer within an image manifest.
type LayerRef struct {
	Digest    string `json:"digest"`
	Size      int64  `json:"size"`
	CreatedBy string `json:"createdBy"`
}

// Image is the on-disk manifest for a built image.
type Image struct {
	Name    string      `json:"name"`
	Tag     string      `json:"tag"`
	Digest  string      `json:"digest"`
	Created string      `json:"created"`
	Config  ImageConfig `json:"config"`
	Layers  []LayerRef  `json:"layers"`
}

// computeDigest serializes the manifest with digest="" and returns the sha256 hash.
func computeDigest(img *Image) (string, error) {
	// Create a copy with empty digest for hashing
	tmp := *img
	tmp.Digest = ""
	data, err := json.Marshal(tmp)
	if err != nil {
		return "", fmt.Errorf("marshal for digest: %w", err)
	}
	return util.HashBytes(data), nil
}

// imageFileName returns the file name used to store this image manifest.
func imageFileName(name, tag string) string {
	// Replace / in names (e.g. library/alpine) with _
	safe := strings.ReplaceAll(name, "/", "_")
	return safe + "_" + tag + ".json"
}

// SaveImage computes the manifest digest and writes it to images/.
func (s *Store) SaveImage(img *Image) error {
	if img.Created == "" {
		img.Created = time.Now().UTC().Format(time.RFC3339)
	}

	digest, err := computeDigest(img)
	if err != nil {
		return err
	}
	img.Digest = digest

	data, err := json.MarshalIndent(img, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal image: %w", err)
	}

	path := filepath.Join(s.ImagesDir(), imageFileName(img.Name, img.Tag))
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write image %s: %w", path, err)
	}
	s.fixOwnership(path)
	return nil
}

// LoadImage reads an image manifest from disk.
func (s *Store) LoadImage(name, tag string) (*Image, error) {
	path := filepath.Join(s.ImagesDir(), imageFileName(name, tag))
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("image %s:%s not found", name, tag)
		}
		return nil, fmt.Errorf("read image: %w", err)
	}

	var img Image
	if err := json.Unmarshal(data, &img); err != nil {
		return nil, fmt.Errorf("unmarshal image: %w", err)
	}
	return &img, nil
}

// ListImages returns all image manifests in the store.
func (s *Store) ListImages() ([]*Image, error) {
	entries, err := os.ReadDir(s.ImagesDir())
	if err != nil {
		return nil, fmt.Errorf("read images dir: %w", err)
	}

	var images []*Image
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.ImagesDir(), e.Name()))
		if err != nil {
			continue
		}
		var img Image
		if err := json.Unmarshal(data, &img); err != nil {
			continue
		}
		images = append(images, &img)
	}
	return images, nil
}

// RemoveImage deletes the manifest for the given image. Does not remove layers.
func (s *Store) RemoveImage(name, tag string) error {
	path := filepath.Join(s.ImagesDir(), imageFileName(name, tag))
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("image %s:%s not found", name, tag)
		}
		return fmt.Errorf("remove image: %w", err)
	}
	return nil
}

// PrintImages prints a formatted table of all images.
func (s *Store) PrintImages() error {
	images, err := s.ListImages()
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NAME\tTAG\tDIGEST\tCREATED\tSIZE")

	for _, img := range images {
		digest := img.Digest
		if len(digest) > 19 {
			digest = digest[:19] // sha256: + first 12 hex chars
		}
		var totalSize int64
		for _, l := range img.Layers {
			totalSize += l.Size
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d B\n", img.Name, img.Tag, digest, img.Created, totalSize)
	}
	return w.Flush()
}
