package build

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"docksmith/internal/container"
	"docksmith/internal/store"
	"docksmith/internal/util"
)

// execFROM loads the base image and initializes the engine state.
func (e *Engine) execFROM(args string) error {
	name, tag := parseImageRef(args)

	img, err := e.store.LoadImage(name, tag)
	if err != nil {
		return fmt.Errorf("FROM: %w", err)
	}

	e.layers = make([]store.LayerRef, len(img.Layers))
	copy(e.layers, img.Layers)

	// Inherit config from base image
	if img.Config.WorkingDir != "" {
		e.workdir = img.Config.WorkingDir
	}
	for _, env := range img.Config.Env {
		e.env = append(e.env, env)
	}

	fmt.Printf(" -> Base image %s:%s (%d layers)\n", name, tag, len(img.Layers))
	return nil
}

// execCOPY copies files from the build context into the working filesystem.
func (e *Engine) execCOPY(args string) error {
	parts := strings.Fields(args)
	if len(parts) < 2 {
		return fmt.Errorf("COPY requires <src> <dest>")
	}

	dest := parts[len(parts)-1]
	srcs := parts[:len(parts)-1]

	// Assemble current filesystem
	rootfs, err := e.assembleRootfs()
	if err != nil {
		return fmt.Errorf("COPY assemble rootfs: %w", err)
	}
	defer os.RemoveAll(rootfs)

	// Ensure WORKDIR creation
	e.ensureWorkdir(rootfs)

	// Resolve destination path
	destPath := dest
	if !filepath.IsAbs(destPath) {
		destPath = filepath.Join(e.workdir, destPath)
	}
	fullDest := filepath.Join(rootfs, destPath)

	// Create snapshot for delta
	snapshot, err := e.snapshotDir(rootfs)
	if err != nil {
		return fmt.Errorf("COPY snapshot: %w", err)
	}
	defer os.RemoveAll(snapshot)

	// Copy each source
	for _, src := range srcs {
		// Resolve globs against build context
		matches, err := filepath.Glob(filepath.Join(e.contextPath, src))
		if err != nil {
			return fmt.Errorf("COPY glob %s: %w", src, err)
		}
		if len(matches) == 0 {
			return fmt.Errorf("COPY: no files match %q", src)
		}

		for _, match := range matches {
			relMatch, _ := filepath.Rel(e.contextPath, match)
			info, err := os.Stat(match)
			if err != nil {
				return fmt.Errorf("COPY stat %s: %w", relMatch, err)
			}

			var targetPath string
			// If dest ends with / or multiple sources, copy into directory
			if strings.HasSuffix(dest, "/") || len(matches) > 1 || len(srcs) > 1 {
				if err := os.MkdirAll(fullDest, 0755); err != nil {
					return fmt.Errorf("COPY mkdir %s: %w", fullDest, err)
				}
				targetPath = filepath.Join(fullDest, filepath.Base(match))
			} else {
				if err := os.MkdirAll(filepath.Dir(fullDest), 0755); err != nil {
					return fmt.Errorf("COPY mkdir parent: %w", err)
				}
				targetPath = fullDest
			}

			if info.IsDir() {
				if err := copyDir(match, targetPath); err != nil {
					return fmt.Errorf("COPY dir %s: %w", relMatch, err)
				}
			} else {
				if err := copyFile(match, targetPath); err != nil {
					return fmt.Errorf("COPY file %s: %w", relMatch, err)
				}
			}
		}
	}

	// Create the delta layer
	tarData, err := util.CreateTarDelta(snapshot, rootfs)
	if err != nil {
		return fmt.Errorf("COPY tar delta: %w", err)
	}

	digest, size, err := e.store.WriteLayer(tarData)
	if err != nil {
		return fmt.Errorf("COPY write layer: %w", err)
	}

	e.layers = append(e.layers, store.LayerRef{
		Digest:    digest,
		Size:      size,
		CreatedBy: "COPY " + args,
	})

	fmt.Printf(" -> Layer %s (%.1f KB)\n", digest[:19], float64(size)/1024)
	return nil
}

// execRUN executes a shell command inside the assembled layer filesystem.
// Uses the shared Isolate() primitive (same as docksmith run).
func (e *Engine) execRUN(args string) error {
	// Assemble current filesystem
	rootfs, err := e.assembleRootfs()
	if err != nil {
		return fmt.Errorf("RUN assemble rootfs: %w", err)
	}
	defer os.RemoveAll(rootfs)

	// Ensure WORKDIR creation
	e.ensureWorkdir(rootfs)

	// Create snapshot for delta
	snapshot, err := e.snapshotDir(rootfs)
	if err != nil {
		return fmt.Errorf("RUN snapshot: %w", err)
	}
	defer os.RemoveAll(snapshot)

	// Build environment using the shared function
	environ := container.BuildEnvironment(e.env, nil)

	// Execute using the shared isolation primitive
	exitCode, err := container.Isolate(rootfs, e.workdir, environ, []string{"/bin/sh", "-c", args})
	if err != nil {
		return fmt.Errorf("RUN command failed: %w", err)
	}
	if exitCode != 0 {
		return fmt.Errorf("RUN command exited with code %d", exitCode)
	}

	// Create the delta layer
	tarData, err := util.CreateTarDelta(snapshot, rootfs)
	if err != nil {
		return fmt.Errorf("RUN tar delta: %w", err)
	}

	digest, size, err := e.store.WriteLayer(tarData)
	if err != nil {
		return fmt.Errorf("RUN write layer: %w", err)
	}

	e.layers = append(e.layers, store.LayerRef{
		Digest:    digest,
		Size:      size,
		CreatedBy: "RUN " + args,
	})

	return nil
}

// execWORKDIR sets the working directory for subsequent instructions.
func (e *Engine) execWORKDIR(args string) error {
	dir := strings.TrimSpace(args)
	if dir == "" {
		return fmt.Errorf("WORKDIR requires a path")
	}
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(e.workdir, dir)
	}
	e.workdir = dir
	fmt.Printf(" -> Working directory: %s\n", e.workdir)
	return nil
}

// execENV adds an environment variable to the image config.
func (e *Engine) execENV(args string) error {
	// Support both "KEY=value" and "KEY value" forms
	if idx := strings.Index(args, "="); idx > 0 {
		e.env = append(e.env, args)
	} else {
		parts := strings.SplitN(args, " ", 2)
		if len(parts) != 2 {
			return fmt.Errorf("ENV requires KEY=value or KEY value")
		}
		e.env = append(e.env, parts[0]+"="+parts[1])
	}
	fmt.Printf(" -> Environment: %s\n", args)
	return nil
}

// execCMD sets the default command for the image.
func (e *Engine) execCMD(args string) error {
	args = strings.TrimSpace(args)
	var cmdSlice []string

	if err := json.Unmarshal([]byte(args), &cmdSlice); err != nil {
		return fmt.Errorf("CMD must be a JSON array, e.g. [\"cmd\",\"arg\"]: %w", err)
	}

	e.cmd = cmdSlice
	fmt.Printf(" -> Default command: %v\n", e.cmd)
	return nil
}

// assembleRootfs creates a temp dir and extracts all current layers into it.
func (e *Engine) assembleRootfs() (string, error) {
	rootfs, err := os.MkdirTemp("", "docksmith-rootfs-*")
	if err != nil {
		return "", fmt.Errorf("create rootfs temp dir: %w", err)
	}

	for _, layer := range e.layers {
		tarData, err := e.store.ReadLayer(layer.Digest)
		if err != nil {
			os.RemoveAll(rootfs)
			return "", fmt.Errorf("read layer %s: %w", layer.Digest[:19], err)
		}
		if err := util.ExtractTar(tarData, rootfs); err != nil {
			os.RemoveAll(rootfs)
			return "", fmt.Errorf("extract layer %s: %w", layer.Digest[:19], err)
		}
	}

	return rootfs, nil
}

// ensureWorkdir creates the WORKDIR in the given rootfs if it doesn't exist.
func (e *Engine) ensureWorkdir(rootfs string) {
	if e.workdir != "" {
		target := filepath.Join(rootfs, e.workdir)
		os.MkdirAll(target, 0755)
	}
}

// snapshotDir creates a recursive copy of a directory tree for diffing.
func (e *Engine) snapshotDir(src string) (string, error) {
	snapshot, err := os.MkdirTemp("", "docksmith-snap-*")
	if err != nil {
		return "", err
	}

	// Use tar to create an efficient copy
	tarData, err := util.CreateTar(src)
	if err != nil {
		os.RemoveAll(snapshot)
		return "", fmt.Errorf("snapshot tar: %w", err)
	}
	if err := util.ExtractTar(tarData, snapshot); err != nil {
		os.RemoveAll(snapshot)
		return "", fmt.Errorf("snapshot extract: %w", err)
	}

	return snapshot, nil
}

// parseImageRef splits "name:tag" into name and tag (defaults to "latest").
func parseImageRef(ref string) (string, string) {
	parts := strings.SplitN(ref, ":", 2)
	name := parts[0]
	tag := "latest"
	if len(parts) == 2 && parts[1] != "" {
		tag = parts[1]
	}
	return name, tag
}

// copyFile copies a single file from src to dst, preserving permissions.
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, info.Mode())
}

// copyDir recursively copies a directory tree.
func copyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// hashBuildContext computes a hash of the files being copied (for cache key).
func hashBuildContext(contextPath string, srcs []string) (string, error) {
	var allData []byte
	for _, src := range srcs {
		matches, err := filepath.Glob(filepath.Join(contextPath, src))
		if err != nil {
			return "", err
		}
		for _, match := range matches {
			err := filepath.Walk(match, func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() {
					return err
				}
				data, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				allData = append(allData, data...)
				return nil
			})
			if err != nil {
				return "", err
			}
		}
	}
	return util.HashBytes(allData), nil
}
