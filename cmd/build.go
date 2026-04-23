package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"docksmith/internal/build"
	"docksmith/internal/store"
)

// BuildCmd implements 'docksmith build'.
func BuildCmd(args []string) error {
	// Parse flags
	file := "Docksmithfile"
	nameTag := ""
	contextDir := "."
	noCache := false

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-f", "--file":
			if i+1 >= len(args) {
				return fmt.Errorf("-f requires a path argument")
			}
			i++
			file = args[i]
		case "-t", "--tag":
			if i+1 >= len(args) {
				return fmt.Errorf("-t requires a name:tag argument")
			}
			i++
			nameTag = args[i]
		case "--no-cache":
			noCache = true
		default:
			contextDir = args[i]
		}
	}

	if nameTag == "" {
		return fmt.Errorf("image name required: use -t name:tag")
	}

	// Parse name:tag
	name, tag := parseRef(nameTag)

	// Resolve paths
	contextDir, err := filepath.Abs(contextDir)
	if err != nil {
		return fmt.Errorf("resolve context dir: %w", err)
	}

	// If file is relative, resolve against context dir
	if !filepath.IsAbs(file) {
		file = filepath.Join(contextDir, file)
	}

	if _, err := os.Stat(file); err != nil {
		return fmt.Errorf("Docksmithfile not found: %s", file)
	}

	// Initialize store
	s, err := store.NewStore()
	if err != nil {
		return fmt.Errorf("init store: %w", err)
	}

	// Create engine and build
	engine, err := build.NewEngine(s, contextDir, noCache)
	if err != nil {
		return fmt.Errorf("init build engine: %w", err)
	}

	_, err = engine.Build(file, name, tag)
	return err
}
