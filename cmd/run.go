package cmd

import (
	"fmt"
	"strings"

	"docksmith/internal/container"
	"docksmith/internal/store"
)

// RunCmd implements 'docksmith run'.
func RunCmd(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: docksmith run [-e KEY=VALUE] <image>[:<tag>] [command...]")
	}

	var envOverrides []string
	var remaining []string

	for i := 0; i < len(args); i++ {
		if args[i] == "-e" {
			if i+1 >= len(args) {
				return fmt.Errorf("-e requires a KEY=VALUE argument")
			}
			i++
			envOverrides = append(envOverrides, args[i])
		} else {
			remaining = append(remaining, args[i])
		}
	}

	if len(remaining) < 1 {
		return fmt.Errorf("usage: docksmith run [-e KEY=VALUE] <image>[:<tag>] [command...]")
	}

	name, tag := parseRef(remaining[0])
	var cmdOverride []string
	if len(remaining) > 1 {
		cmdOverride = remaining[1:]
	}

	s, err := store.NewStore()
	if err != nil {
		return fmt.Errorf("init store: %w", err)
	}

	return container.Run(s, name, tag, cmdOverride, envOverrides)
}

// parseRef splits "name:tag" into name and tag (defaults to "latest").
func parseRef(ref string) (string, string) {
	parts := strings.SplitN(ref, ":", 2)
	name := parts[0]
	tag := "latest"
	if len(parts) == 2 && parts[1] != "" {
		tag = parts[1]
	}
	return name, tag
}
