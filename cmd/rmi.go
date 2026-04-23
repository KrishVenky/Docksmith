package cmd

import (
	"fmt"

	"docksmith/internal/store"
)

// RmiCmd implements 'docksmith rmi'.
// Removes the image manifest AND all of its layer files from disk.
func RmiCmd(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: docksmith rmi <image>[:<tag>]")
	}

	name, tag := parseRef(args[0])

	s, err := store.NewStore()
	if err != nil {
		return fmt.Errorf("init store: %w", err)
	}

	// Load image first to get layer list
	img, err := s.LoadImage(name, tag)
	if err != nil {
		return err
	}

	// Remove layer files (no reference counting per spec)
	for _, layer := range img.Layers {
		if err := s.RemoveLayer(layer.Digest); err != nil {
			// Log but don't fail — layer may already be gone
			fmt.Printf("Warning: could not remove layer %s: %v\n", layer.Digest[:19], err)
		}
	}

	// Remove the manifest
	if err := s.RemoveImage(name, tag); err != nil {
		return err
	}

	fmt.Printf("Removed image %s:%s (%d layers deleted)\n", name, tag, len(img.Layers))
	return nil
}
