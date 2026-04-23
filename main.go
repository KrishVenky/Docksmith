package main

import (
	"fmt"
	"os"

	"docksmith/cmd"
)

const usage = `Docksmith — a simplified Docker-like build and runtime system

Usage:
  docksmith <command> [arguments]

Commands:
  build     Build an image from a Docksmithfile
  run       Run a command in a new container
  images    List images
  rmi       Remove an image and its layers
  cache     Show build cache entries
  import    Import a base image from a rootfs tarball

Build Usage:
  docksmith build -t <name:tag> [-f Docksmithfile] [--no-cache] [context_dir]

Run Usage:
  docksmith run [-e KEY=VALUE] <image>[:<tag>] [command...]

Import Usage:
  docksmith import <name>[:<tag>] <rootfs.tar>
`

func main() {
	if len(os.Args) < 2 {
		fmt.Print(usage)
		os.Exit(1)
	}

	command := os.Args[1]
	args := os.Args[2:]

	var err error
	switch command {
	case "build":
		err = cmd.BuildCmd(args)
	case "run":
		err = cmd.RunCmd(args)
	case "images":
		err = cmd.ImagesCmd(args)
	case "rmi":
		err = cmd.RmiCmd(args)
	case "cache":
		err = cmd.CacheCmd(args)
	case "import":
		err = cmd.ImportCmd(args)
	case "help", "-h", "--help":
		fmt.Print(usage)
		return
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", command)
		fmt.Print(usage)
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
