package container

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"docksmith/internal/store"
	"docksmith/internal/util"
)

// Run starts a container from the given image.
// If cmdOverride is non-empty, it replaces the image's default CMD.
// envOverrides are -e KEY=VALUE pairs that take precedence over image ENV.
func Run(s *store.Store, name, tag string, cmdOverride []string, envOverrides []string) error {
	img, err := s.LoadImage(name, tag)
	if err != nil {
		return fmt.Errorf("load image: %w", err)
	}

	// Determine the command to run
	cmdSlice := img.Config.Cmd
	if len(cmdOverride) > 0 {
		cmdSlice = cmdOverride
	}
	if len(cmdSlice) == 0 {
		return fmt.Errorf("no command specified and image %s:%s has no CMD defined", name, tag)
	}

	// Create container rootfs
	rootfs, err := os.MkdirTemp("", "docksmith-container-*")
	if err != nil {
		return fmt.Errorf("create container rootfs: %w", err)
	}
	defer os.RemoveAll(rootfs)

	// Extract all layers in order
	fmt.Printf("Preparing container from %s:%s (%d layers)...\n", name, tag, len(img.Layers))
	for _, layer := range img.Layers {
		tarData, err := s.ReadLayer(layer.Digest)
		if err != nil {
			return fmt.Errorf("read layer %s: %w", layer.Digest[:19], err)
		}
		if err := util.ExtractTar(tarData, rootfs); err != nil {
			return fmt.Errorf("extract layer %s: %w", layer.Digest[:19], err)
		}
	}

	// Build environment from image config
	environ := BuildEnvironment(img.Config.Env, envOverrides)

	// Resolve working directory
	workdir := img.Config.WorkingDir
	if workdir == "" {
		workdir = "/"
	}
	os.MkdirAll(rootfs+workdir, 0755)

	fmt.Printf("Running: %s\n", strings.Join(cmdSlice, " "))

	// Execute with shared isolation primitive
	exitCode, err := Isolate(rootfs, workdir, environ, cmdSlice)
	if err != nil {
		return err
	}

	fmt.Printf("Container exited with code %d\n", exitCode)
	if exitCode != 0 {
		os.Exit(exitCode)
	}
	return nil
}

// Isolate is the shared isolation primitive used by both 'docksmith run' and
// 'RUN' during build. It chroots into rootfs, sets the working directory and
// environment, and execs the given command.
// Returns the exit code and any error.
func Isolate(rootfs, workdir string, environ []string, cmdSlice []string) (int, error) {
	executable := cmdSlice[0]
	var cmdArgs []string
	if len(cmdSlice) > 1 {
		cmdArgs = cmdSlice[1:]
	}

	// Resolve the executable path within the rootfs
	execPath := resolveExecutable(rootfs, executable, environ)
	if execPath == "" {
		execPath = executable
	}

	cmd := exec.Command(execPath, cmdArgs...)
	cmd.Dir = workdir
	cmd.Env = environ
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Chroot: rootfs,
	}

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode(), nil
		}
		return 1, fmt.Errorf("container process error: %w", err)
	}
	return 0, nil
}

// BuildEnvironment constructs the environment variable list.
// Image env values are the base, then overrides take precedence.
func BuildEnvironment(imageEnv []string, overrides []string) []string {
	envMap := map[string]string{
		"PATH": "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"HOME": "/root",
		"TERM": "xterm",
	}

	// Apply image ENV
	for _, e := range imageEnv {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}

	// Apply -e overrides (take precedence)
	for _, e := range overrides {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}

	var environ []string
	for k, v := range envMap {
		environ = append(environ, k+"="+v)
	}
	return environ
}

// resolveExecutable searches for the executable in the rootfs PATH directories.
func resolveExecutable(rootfs, executable string, environ []string) string {
	if strings.HasPrefix(executable, "/") {
		if _, err := os.Stat(rootfs + executable); err == nil {
			return executable
		}
		return ""
	}

	pathDirs := "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
	for _, env := range environ {
		if strings.HasPrefix(env, "PATH=") {
			pathDirs = strings.TrimPrefix(env, "PATH=")
			break
		}
	}

	for _, dir := range strings.Split(pathDirs, ":") {
		candidate := rootfs + dir + "/" + executable
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return dir + "/" + executable
		}
	}

	return ""
}
