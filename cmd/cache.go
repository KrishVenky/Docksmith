package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"docksmith/internal/build"
	"docksmith/internal/store"
)

// CacheCmd implements 'docksmith cache'.
func CacheCmd(args []string) error {
	s, err := store.NewStore()
	if err != nil {
		return fmt.Errorf("init store: %w", err)
	}

	cache, err := build.NewCache(s)
	if err != nil {
		return fmt.Errorf("init cache: %w", err)
	}

	entries := cache.Entries()
	if len(entries) == 0 {
		fmt.Println("Build cache is empty.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "CACHE KEY\tLAYER DIGEST")
	for key, digest := range entries {
		short := func(s string) string {
			if len(s) > 19 {
				return s[:19]
			}
			return s
		}
		fmt.Fprintf(w, "%s\t%s\n", short(key), short(digest))
	}
	fmt.Fprintf(w, "\nTotal entries: %d\n", len(entries))
	return w.Flush()
}
