package store

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"syscall"
)

// Store manages the ~/.docksmith/ state directory.
type Store struct {
	Root string
	uid  int // owner UID (-1 if not needed)
	gid  int // owner GID (-1 if not needed)
}

// NewStore creates a Store and ensures all subdirectories exist.
// When running under sudo, it resolves the original user's home directory
// and chowns created files/dirs so they remain accessible without sudo.
func NewStore() (*Store, error) {
	home := ""
	uid := -1
	gid := -1

	if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" {
		u, err := user.Lookup(sudoUser)
		if err != nil {
			return nil, fmt.Errorf("lookup SUDO_USER %s: %w", sudoUser, err)
		}
		home = u.HomeDir
		uid, _ = strconv.Atoi(u.Uid)
		gid, _ = strconv.Atoi(u.Gid)
	} else {
		var err error
		home, err = os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("get home dir: %w", err)
		}
	}

	root := filepath.Join(home, ".docksmith")
	s := &Store{Root: root, uid: uid, gid: gid}

	for _, dir := range []string{s.Root, s.ImagesDir(), s.LayersDir(), s.CacheDir()} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("create dir %s: %w", dir, err)
		}
		s.fixOwnership(dir)
	}
	return s, nil
}

// fixOwnership changes ownership of a path to the real user when running under sudo.
func (s *Store) fixOwnership(path string) {
	if s.uid >= 0 && s.gid >= 0 {
		syscall.Chown(path, s.uid, s.gid)
	}
}

// ImagesDir returns the path to images/.
func (s *Store) ImagesDir() string {
	return filepath.Join(s.Root, "images")
}

// LayersDir returns the path to layers/.
func (s *Store) LayersDir() string {
	return filepath.Join(s.Root, "layers")
}

// CacheDir returns the path to cache/.
func (s *Store) CacheDir() string {
	return filepath.Join(s.Root, "cache")
}
