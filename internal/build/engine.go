package build

import (
	"fmt"
	"strings"
	"time"

	"docksmith/internal/store"
)

// Engine orchestrates the build process.
type Engine struct {
	store       *store.Store
	cache       *Cache
	contextPath string
	noCache     bool

	// Build state
	layers  []store.LayerRef
	env     []string
	workdir string
	cmd     []string

	// configState tracks non-layer config for cache invalidation.
	// Changes to WORKDIR or ENV affect subsequent cache keys.
	configState string
}

// NewEngine creates a build engine.
func NewEngine(s *store.Store, contextPath string, noCache bool) (*Engine, error) {
	c, err := NewCache(s)
	if err != nil {
		return nil, fmt.Errorf("init cache: %w", err)
	}
	return &Engine{
		store:       s,
		cache:       c,
		contextPath: contextPath,
		noCache:     noCache,
		workdir:     "/",
	}, nil
}

// Build parses the Docksmithfile and executes all instructions, producing an image.
func (e *Engine) Build(docksmithfilePath, name, tag string) (*store.Image, error) {
	instructions, err := Parse(docksmithfilePath)
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}

	totalStart := time.Now()
	allCacheHits := true

	fmt.Printf("Building %s:%s from %s\n", name, tag, docksmithfilePath)

	for i, inst := range instructions {
		stepStart := time.Now()

		// FROM has special output: no cache status, no timing
		if inst.Command == "FROM" {
			fmt.Printf("\nStep %d/%d : %s %s\n", i+1, len(instructions), inst.Command, inst.Args)
			if err := e.executeInstruction(inst); err != nil {
				return nil, fmt.Errorf("step %d (line %d): %w", i+1, inst.Line, err)
			}
			continue
		}

		// Non-layer-producing instructions (WORKDIR, ENV, CMD)
		if inst.Command == "WORKDIR" || inst.Command == "ENV" || inst.Command == "CMD" {
			fmt.Printf("\nStep %d/%d : %s %s\n", i+1, len(instructions), inst.Command, inst.Args)
			if err := e.executeInstruction(inst); err != nil {
				return nil, fmt.Errorf("step %d (line %d): %w", i+1, inst.Line, err)
			}
			// Update config state to affect subsequent cache keys
			e.updateConfigState()
			continue
		}

		// Layer-producing instructions: COPY, RUN
		parentDigest := e.chainDigest()
		contentHash := ""

		if inst.Command == "COPY" {
			parts := strings.Fields(inst.Args)
			if len(parts) >= 2 {
				srcs := parts[:len(parts)-1]
				ch, err := hashBuildContext(e.contextPath, srcs)
				if err == nil {
					contentHash = ch
				}
			}
		}

		cacheKey := ComputeCacheKey(parentDigest, inst.Command+" "+inst.Args, contentHash)

		// Check cache (unless --no-cache)
		if !e.noCache {
			if layerDigest, ok := e.cache.Lookup(cacheKey); ok {
				if e.store.LayerExists(layerDigest) {
					tarData, err := e.store.ReadLayer(layerDigest)
					if err == nil {
						elapsed := time.Since(stepStart)
						fmt.Printf("\nStep %d/%d : %s %s [CACHE HIT] %.2fs\n",
							i+1, len(instructions), inst.Command, inst.Args, elapsed.Seconds())
						e.layers = append(e.layers, store.LayerRef{
							Digest:    layerDigest,
							Size:      int64(len(tarData)),
							CreatedBy: inst.Command + " " + inst.Args,
						})
						continue
					}
				}
				// Layer missing from disk — invalidate (cascade handled by chain digest)
			}
		}

		// Cache miss — execute the instruction
		allCacheHits = false
		fmt.Printf("\nStep %d/%d : %s %s", i+1, len(instructions), inst.Command, inst.Args)

		if err := e.executeInstruction(inst); err != nil {
			return nil, fmt.Errorf("step %d (line %d): %w", i+1, inst.Line, err)
		}

		elapsed := time.Since(stepStart)
		fmt.Printf(" [CACHE MISS] %.2fs\n", elapsed.Seconds())

		// Cache the result
		if len(e.layers) > 0 {
			lastLayer := e.layers[len(e.layers)-1]
			if !e.noCache {
				if err := e.cache.Store(cacheKey, lastLayer.Digest); err != nil {
					fmt.Printf(" -> Warning: failed to cache layer: %v\n", err)
				}
			}
		}
	}

	// Determine the created timestamp
	created := time.Now().UTC().Format(time.RFC3339)
	if allCacheHits {
		// Preserve original timestamp when all steps are cache hits
		existing, err := e.store.LoadImage(name, tag)
		if err == nil && existing.Created != "" {
			created = existing.Created
		}
	}

	// Assemble the final image manifest
	img := &store.Image{
		Name:    name,
		Tag:     tag,
		Created: created,
		Config: store.ImageConfig{
			Env:        e.env,
			Cmd:        e.cmd,
			WorkingDir: e.workdir,
		},
		Layers: e.layers,
	}

	if err := e.store.SaveImage(img); err != nil {
		return nil, fmt.Errorf("save image: %w", err)
	}

	totalElapsed := time.Since(totalStart)
	fmt.Printf("\nSuccessfully built %s %s:%s (%.2fs)\n", img.Digest[:19], name, tag, totalElapsed.Seconds())
	return img, nil
}

// executeInstruction dispatches to the appropriate handler.
func (e *Engine) executeInstruction(inst Instruction) error {
	switch inst.Command {
	case "FROM":
		return e.execFROM(inst.Args)
	case "COPY":
		return e.execCOPY(inst.Args)
	case "RUN":
		return e.execRUN(inst.Args)
	case "WORKDIR":
		return e.execWORKDIR(inst.Args)
	case "ENV":
		return e.execENV(inst.Args)
	case "CMD":
		return e.execCMD(inst.Args)
	default:
		return fmt.Errorf("unknown instruction: %s", inst.Command)
	}
}

// chainDigest computes a combined digest of all current layer digests +
// config state (WORKDIR, ENV). This ensures WORKDIR/ENV changes invalidate
// subsequent cache keys.
func (e *Engine) chainDigest() string {
	var parts []string
	for _, l := range e.layers {
		parts = append(parts, l.Digest)
	}
	if e.configState != "" {
		parts = append(parts, e.configState)
	}
	if len(parts) == 0 {
		return "sha256:empty"
	}
	return fmt.Sprintf("chain:%s", strings.Join(parts, "+"))
}

// updateConfigState recomputes the config state hash from current WORKDIR and ENV.
func (e *Engine) updateConfigState() {
	raw := "workdir:" + e.workdir + "|env:" + strings.Join(e.env, ",")
	e.configState = raw
}
