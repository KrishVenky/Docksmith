package cmd

import (
	"fmt"

	"docksmith/internal/store"
)

// ImagesCmd implements 'docksmith images'.
func ImagesCmd(args []string) error {
	s, err := store.NewStore()
	if err != nil {
		return fmt.Errorf("init store: %w", err)
	}

	return s.PrintImages()
}
