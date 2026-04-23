package cmd

import (
	"fmt"
	"os"

	"docksmith/internal/store"
	"docksmith/internal/util"
)

// ImportCmd implements 'docksmith import'.
// Usage: docksmith import <name>[:<tag>] <rootfs.tar>
func ImportCmd(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: docksmith import <name>[:<tag>] <rootfs.tar>")
	}

	name, tag := parseRef(args[0])
	tarPath := args[1]

	// Read the tar file
	tarData, err := os.ReadFile(tarPath)
	if err != nil {
		return fmt.Errorf("read tar %s: %w", tarPath, err)
	}

	fmt.Printf("Importing %s (%d bytes) as %s:%s...\n", tarPath, len(tarData), name, tag)

	// Initialize store
	s, err := store.NewStore()
	if err != nil {
		return fmt.Errorf("init store: %w", err)
	}

	// Write the entire tarball as a single layer
	digest, size, err := s.WriteLayer(tarData)
	if err != nil {
		return fmt.Errorf("write layer: %w", err)
	}

	// Determine createdBy description
	createdBy := fmt.Sprintf("imported from %s", tarPath)

	// Check if we should try to split into multiple layers
	// For simplicity, import creates a single layer from the entire tarball.
	// This matches how base images typically work.
	img := &store.Image{
		Name: name,
		Tag:  tag,
		Config: store.ImageConfig{
			Env:        []string{},
			Cmd:        []string{"/bin/sh"},
			WorkingDir: "/",
		},
		Layers: []store.LayerRef{
			{
				Digest:    digest,
				Size:      size,
				CreatedBy: createdBy,
			},
		},
	}

	if err := s.SaveImage(img); err != nil {
		return fmt.Errorf("save image: %w", err)
	}

	fmt.Printf("Successfully imported %s:%s\n", name, tag)
	fmt.Printf("  Layer: %s (%.1f MB)\n", digest[:19], float64(size)/1024/1024)
	fmt.Printf("  Digest: %s\n", img.Digest)

	// Verify by re-reading
	_, err = s.LoadImage(name, tag)
	if err != nil {
		return fmt.Errorf("verification failed: %w", err)
	}

	// Quick integrity check: verify the layer hash
	storedData, err := s.ReadLayer(digest)
	if err != nil {
		return fmt.Errorf("layer integrity check failed: %w", err)
	}
	verifyDigest := util.HashBytes(storedData)
	if verifyDigest != digest {
		return fmt.Errorf("layer integrity mismatch: expected %s, got %s", digest, verifyDigest)
	}

	return nil
}
