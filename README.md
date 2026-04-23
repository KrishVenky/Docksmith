# Docksmith

A simplified Docker-like build and runtime system with both Go and C implementations. Supports building container images from a Docksmithfile, importing base images from rootfs tarballs, and running isolated containers.

## Requirements

- **Linux environment required** (WSL2 on Windows recommended)
- **Ubuntu/Kali**: `sudo apt install -y build-essential pkg-config libssl-dev golang`
- **macOS**: `brew install gcc pkg-config openssl go`

**Note**: This project uses Linux-specific isolation primitives (chroot, fork/exec). It will not run on native Windows or macOS without WSL/equivalent virtualization.

## Quick Start (WSL/Linux)

```bash
# 1. Copy project to Linux filesystem (not /mnt/c)
mkdir -p ~/src
cp -a /path/to/docksmith ~/src/

cd ~/src/docksmith

# 2. Build both binaries
make
go build -o docksmith .

# 3. One-command smoke test (requires alpine-minirootfs-3.19.1-x86_64.tar)
make smoke

# Or for C binary
make smoke-c
```

## Full Workflow Example

```bash
# Import base image
sudo ./docksmith import alpine:3.19 alpine-minirootfs-3.19.1-x86_64.tar

# Build image from Docksmithfile
sudo ./docksmith build -t myapp:v1 -f Docksmithfile .

# List images
sudo ./docksmith images

# Run container
sudo ./docksmith run -e GREETING=hello myapp:v1

# View build cache
sudo ./docksmith cache

# Remove image
sudo ./docksmith rmi myapp:v1
```

## CLI Commands

### import
Import a base image from a rootfs tarball.
```bash
./docksmith import <name>[:<tag>] <rootfs.tar>

# Example
./docksmith import alpine:3.19 alpine-minirootfs-3.19.1-x86_64.tar
```

### build
Build an image from a Docksmithfile.
```bash
./docksmith build -t <name:tag> [-f Docksmithfile] [--no-cache] [context_dir]

# Examples
./docksmith build -t demo:latest
./docksmith build -t myapp:v1 -f Docksmithfile ./sample-app
./docksmith build -t myapp:v2 --no-cache .
```

### run
Run a command in a new container.
```bash
./docksmith run [-e KEY=VALUE] <image>[:<tag>] [command...]

# Examples
./docksmith run demo:latest                     # Use image CMD
./docksmith run -e GREETING=hello demo /bin/sh # Override with command
./docksmith run -e A=1 -e B=2 demo:v1 sh -c "echo $A $B"
```

### images
List all images.
```bash
./docksmith images
```

### cache
Show build cache entries.
```bash
./docksmith cache
```

### rmi
Remove an image and its layers.
```bash
./docksmith rmi <image>[:<tag>]

# Examples
./docksmith rmi demo:v1
./docksmith rmi myapp    # defaults to latest tag
```

## Docksmithfile Syntax

A Docksmithfile is similar to Dockerfile.

```dockerfile
FROM alpine:3.19
WORKDIR /app
COPY main.sh /app/main.sh
RUN chmod +x /app/main.sh
ENV GREETING=hello
CMD ["/bin/sh", "/app/main.sh"]
```

**Supported instructions:**
- `FROM <image>[:<tag>]` — Base image (required)
- `WORKDIR <dir>` — Set working directory
- `COPY <src> <dest>` — Copy file/directory from context into image
- `RUN <command>` — Execute command in container (creates layer)
- `ENV <key>=<value>` — Set environment variable
- `CMD [<cmd>, <arg>, ...]` — Default command to run (JSON array format)

## Project Structure

```
.
├── README.md                 # This file
├── Docksmithfile             # Demo Docksmithfile
├── main.go                   # Go CLI entrypoint
├── main.sh                   # Demo container script
├── Makefile                  # Build system
├── go.mod                    # Go module definition
├── .gitignore                # Ignore patterns
│
├── cmd/                      # Go CLI handlers
│   ├── build.go
│   ├── cache.go
│   ├── images.go
│   ├── import.go
│   ├── rmi.go
│   └── run.go
│
├── internal/                 # Go implementation
│   ├── build/
│   │   ├── cache.go
│   │   ├── engine.go
│   │   ├── instructions.go
│   │   └── parser.go
│   ├── container/
│   │   └── run.go
│   ├── store/
│   │   ├── image.go
│   │   ├── layer.go
│   │   └── store.go
│   └── util/
│       ├── hash.go
│       └── tar.go
│
├── c_src/                    # C implementation (docksmith_c)
│   ├── main.c
│   ├── build/
│   │   ├── cache.c
│   │   ├── engine.c
│   │   ├── parser.c
│   │   └── .h headers
│   ├── container/
│   │   └── run.c
│   ├── store/
│   │   ├── image.c
│   │   ├── layer.c
│   │   ├── store.c
│   │   └── .h headers
│   ├── cmd/
│   │   ├── commands.c
│   │   └── .h headers
│   └── util/
│       ├── hash.c
│       ├── tar.c
│       └── .h headers
│
├── vendor/                   # cJSON library
│   └── cjson/
│       ├── cJSON.c
│       └── cJSON.h
│
├── testdata/                 # Test fixtures
│   └── context/
│
└── sample-app/               # Example project
    ├── app.sh
    └── Docksmithfile
```

## State/Database

Docksmith stores all state under `~/.docksmith/`:

```
~/.docksmith/
├── images/        # Image metadata (JSON)
│   ├── alpine_3.19.json
│   └── demo_v1.json
├── layers/        # Layer tar archives (indexed by SHA256)
│   ├── sha256-<hash1>
│   ├── sha256-<hash2>
│   └── sha256-<hash3>
└── cache/         # Build cache index
    └── index.json
```

### Inspect database

```bash
# View all files
find ~/.docksmith -maxdepth 3 -type f | sort

# View structure
ls -R ~/.docksmith

# View specific image metadata
cat ~/.docksmith/images/demo_v1.json

# View cache index
cat ~/.docksmith/cache/index.json
```

## Building

### Go binary (docksmith)
```bash
go build -o docksmith .
```

### C binary (docksmith_c)
```bash
make
```

### Both binaries
```bash
make
go build -o docksmith .
```

### Clean
```bash
make clean          # Clean C artifacts
make clean-all      # Clean C artifacts and Go binary
```

## Smoke Tests

One-command validation:

```bash
# Go binary
make smoke

# C binary
make smoke-c

# Shell script (parameterizable)
BASE_IMAGE=alpine:3.19 BASE_TAR=alpine-minirootfs-3.19.1-x86_64.tar ./scripts/wsl-smoke.sh
```

These verify:
- Import, build, run, and cache operations
- Prints images, cache contents, and full store tree

## Running on WSL (Recommended)

1. **Copy to Linux filesystem** (not /mnt/c for better chroot support):
   ```bash
   mkdir -p ~/src
   cp -a /mnt/c/Users/krish/vscode/docksmith ~/src/
   cd ~/src/docksmith
   ```

2. **Install deps** (one-time):
   ```bash
   sudo apt update
   sudo apt install -y build-essential pkg-config libssl-dev golang
   ```

3. **Build and test**:
   ```bash
   make
   go build -o docksmith .
   make smoke
   ```

4. **For container operations** (build/run/exec), use sudo:
   ```bash
   sudo ./docksmith import alpine:3.19 alpine-minirootfs-3.19.1-x86_64.tar
   sudo ./docksmith build -t demo:v1 -f Docksmithfile .
   sudo ./docksmith run -e GREETING=hello demo:v1
   ```

## Troubleshooting

### "fork/exec: operation not permitted"
- **Cause**: Running from /mnt/c in WSL with insufficient isolation permissions
- **Fix**: Copy to Linux home directory (~) and rebuild

### "image not found"
- **Cause**: Referenced image not imported yet
- **Fix**: Import the base image first: `sudo ./docksmith import <name> <tar>`

### Build fails with "no such file or directory"
- **Cause**: Missing dependencies (openssl, gcc, go)
- **Fix**: Run install command above for your OS

### CMAKE_BUILD_TYPE or OpenSSL warnings
- Non-critical compiler warnings; binary still works
- Future C refactors will eliminate these

## Development

### Modifying Docksmithfile
- Update [Docksmithfile](Docksmithfile) with new instructions
- Rebuild: `sudo ./docksmith build --no-cache -t demo:v1 -f Docksmithfile .`

### Modifying Go code
- Edit files in `cmd/` or `internal/`
- Rebuild: `go build -o docksmith .`

### Modifying C code
- Edit files in `c_src/`
- Rebuild: `make clean && make`

### Run tests (Go only)
```bash
go test ./...
```

## License

[Your license here]

## Contributing

[Your contribution guidelines here]
