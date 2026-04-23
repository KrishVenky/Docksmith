package util

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// walkLstat walks a directory tree using Lstat (does not follow symlinks).
// This is critical for correctly snapshotting container filesystems.
func walkLstat(root string, fn func(path string, info os.FileInfo, err error) error) error {
	info, err := os.Lstat(root)
	if err != nil {
		return fn(root, nil, err)
	}
	return walkLstatRecursive(root, info, fn)
}

func walkLstatRecursive(path string, info os.FileInfo, fn func(string, os.FileInfo, error) error) error {
	if err := fn(path, info, nil); err != nil {
		return err
	}
	if !info.IsDir() {
		return nil
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return fn(path, info, err)
	}
	// Sort entries for reproducible tar archives
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})
	for _, entry := range entries {
		childPath := filepath.Join(path, entry.Name())
		childInfo, err := os.Lstat(childPath)
		if err != nil {
			if err2 := fn(childPath, nil, err); err2 != nil {
				return err2
			}
			continue
		}
		if err := walkLstatRecursive(childPath, childInfo, fn); err != nil {
			return err
		}
	}
	return nil
}

// CreateTar creates a tar archive from the given base directory.
// Uses Lstat to correctly preserve symlinks.
func CreateTar(baseDir string) ([]byte, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	err := walkLstat(baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(baseDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}

		return writeEntry(tw, path, rel, info)
	})
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", baseDir, err)
	}

	if err := tw.Close(); err != nil {
		return nil, fmt.Errorf("close tar: %w", err)
	}

	return buf.Bytes(), nil
}

// CreateTarFromPaths creates a tar archive of specific paths relative to baseDir.
func CreateTarFromPaths(baseDir string, paths []string) ([]byte, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	for _, p := range paths {
		absPath := filepath.Join(baseDir, p)
		info, err := os.Lstat(absPath)
		if err != nil {
			return nil, fmt.Errorf("lstat %s: %w", p, err)
		}

		if info.IsDir() {
			err := walkLstat(absPath, func(walkPath string, wInfo os.FileInfo, wErr error) error {
				if wErr != nil {
					return wErr
				}
				rel, err := filepath.Rel(baseDir, walkPath)
				if err != nil {
					return err
				}
				return writeEntry(tw, walkPath, rel, wInfo)
			})
			if err != nil {
				return nil, err
			}
		} else {
			if err := writeEntry(tw, absPath, p, info); err != nil {
				return nil, err
			}
		}
	}

	if err := tw.Close(); err != nil {
		return nil, fmt.Errorf("close tar: %w", err)
	}
	return buf.Bytes(), nil
}

// writeEntry writes a single file/dir/symlink entry to the tar writer.
// absPath is the absolute path on disk, rel is the path inside the tar.
func writeEntry(tw *tar.Writer, absPath, rel string, info os.FileInfo) error {
	link := ""
	if info.Mode()&os.ModeSymlink != 0 {
		var err error
		link, err = os.Readlink(absPath)
		if err != nil {
			return fmt.Errorf("readlink %s: %w", rel, err)
		}
	}

	header, err := tar.FileInfoHeader(info, link)
	if err != nil {
		return fmt.Errorf("tar header %s: %w", rel, err)
	}
	header.Name = filepath.ToSlash(rel)
	if info.IsDir() {
		header.Name += "/"
	}

	// Zero timestamps for reproducible builds
	header.ModTime = time.Time{}
	header.AccessTime = time.Time{}
	header.ChangeTime = time.Time{}

	if err := tw.WriteHeader(header); err != nil {
		return fmt.Errorf("write header %s: %w", rel, err)
	}

	// Only write content for regular files (not symlinks, not dirs)
	if !info.Mode().IsRegular() {
		return nil
	}

	f, err := os.Open(absPath)
	if err != nil {
		return fmt.Errorf("open %s: %w", rel, err)
	}
	defer f.Close()
	if _, err := io.Copy(tw, f); err != nil {
		return fmt.Errorf("write %s: %w", rel, err)
	}

	return nil
}

// CreateTarDelta compares newRoot against oldRoot and creates a tar
// containing only added or modified files in newRoot.
func CreateTarDelta(oldRoot, newRoot string) ([]byte, error) {
	var changedPaths []string

	err := walkLstat(newRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(newRoot, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}

		oldPath := filepath.Join(oldRoot, rel)
		oldInfo, oldErr := os.Lstat(oldPath)

		if oldErr != nil {
			// New file/dir/symlink
			changedPaths = append(changedPaths, rel)
			return nil
		}

		// If types differ (e.g., regular -> symlink), it's changed
		if info.Mode().Type() != oldInfo.Mode().Type() {
			changedPaths = append(changedPaths, rel)
			return nil
		}

		if info.IsDir() {
			return nil
		}

		// For symlinks, compare targets
		if info.Mode()&os.ModeSymlink != 0 {
			newTarget, _ := os.Readlink(path)
			oldTarget, _ := os.Readlink(oldPath)
			if newTarget != oldTarget {
				changedPaths = append(changedPaths, rel)
			}
			return nil
		}

		// For regular files, compare by content hash
		if info.Size() != oldInfo.Size() {
			changedPaths = append(changedPaths, rel)
		} else {
			newHash, err1 := HashFile(path)
			oldHash, err2 := HashFile(oldPath)
			if err1 != nil || err2 != nil || newHash != oldHash {
				changedPaths = append(changedPaths, rel)
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("walk delta: %w", err)
	}

	if len(changedPaths) == 0 {
		var buf bytes.Buffer
		tw := tar.NewWriter(&buf)
		tw.Close()
		return buf.Bytes(), nil
	}

	return CreateTarFromPaths(newRoot, changedPaths)
}

// ExtractTar extracts a tar archive from bytes into destDir.
func ExtractTar(tarData []byte, destDir string) error {
	return ExtractTarReader(bytes.NewReader(tarData), destDir)
}

// ExtractTarFile extracts a tar file from disk into destDir.
func ExtractTarFile(tarPath, destDir string) error {
	f, err := os.Open(tarPath)
	if err != nil {
		return fmt.Errorf("open tar %s: %w", tarPath, err)
	}
	defer f.Close()
	return ExtractTarReader(f, destDir)
}

// ExtractTarReader extracts a tar from an io.Reader into destDir.
func ExtractTarReader(r io.Reader, destDir string) error {
	tr := tar.NewReader(r)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar: %w", err)
		}

		target := filepath.Join(destDir, filepath.Clean(header.Name))

		// Prevent path traversal
		if !strings.HasPrefix(target, filepath.Clean(destDir)+string(os.PathSeparator)) && target != filepath.Clean(destDir) {
			return fmt.Errorf("tar entry %q escapes destination", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(header.Mode)|0755); err != nil {
				return fmt.Errorf("mkdir %s: %w", target, err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return fmt.Errorf("mkdir parent %s: %w", target, err)
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("create %s: %w", target, err)
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return fmt.Errorf("write %s: %w", target, err)
			}
			f.Close()
		case tar.TypeSymlink:
			os.Remove(target)
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return fmt.Errorf("mkdir parent %s: %w", target, err)
			}
			if err := os.Symlink(header.Linkname, target); err != nil {
				return fmt.Errorf("symlink %s: %w", target, err)
			}
		case tar.TypeLink:
			linkTarget := filepath.Join(destDir, filepath.Clean(header.Linkname))
			os.Remove(target)
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return fmt.Errorf("mkdir parent %s: %w", target, err)
			}
			if err := os.Link(linkTarget, target); err != nil {
				return fmt.Errorf("hardlink %s: %w", target, err)
			}
		default:
			continue
		}
	}
	return nil
}
