package build

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"docksmith/internal/store"
	"docksmith/internal/util"
)

// CacheIndex stores mappings from cache keys to layer digests.
type CacheIndex struct {
	Entries map[string]string `json:"entries"`
}

// Cache manages the build cache on disk.
type Cache struct {
	mu    sync.Mutex
	path  string
	index CacheIndex
}

// NewCache loads or creates the cache index.
func NewCache(s *store.Store) (*Cache, error) {
	path := filepath.Join(s.CacheDir(), "index.json")
	c := &Cache{
		path:  path,
		index: CacheIndex{Entries: make(map[string]string)},
	}

	data, err := os.ReadFile(path)
	if err == nil {
		if err := json.Unmarshal(data, &c.index); err != nil {
			// Corrupt cache — start fresh
			c.index = CacheIndex{Entries: make(map[string]string)}
		}
	}

	return c, nil
}

// ComputeCacheKey returns a SHA-256 based cache key for an instruction.
// parentDigest is the digest chain of all previous layers.
// instruction is the raw instruction text (e.g. "COPY . /app").
// contentHash is an optional hash of the input content (for COPY).
func ComputeCacheKey(parentDigest, instruction, contentHash string) string {
	raw := parentDigest + "|" + instruction + "|" + contentHash
	return util.HashBytes([]byte(raw))
}

// Lookup checks if a cache entry exists for the given key.
// Returns the layer digest and true if found.
func (c *Cache) Lookup(key string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	digest, ok := c.index.Entries[key]
	return digest, ok
}

// Store writes a cache entry and persists the index.
func (c *Cache) Store(key, digest string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.index.Entries[key] = digest
	return c.save()
}

func (c *Cache) save() error {
	data, err := json.MarshalIndent(c.index, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal cache: %w", err)
	}
	return os.WriteFile(c.path, data, 0644)
}

// Entries returns a copy of all cache entries.
func (c *Cache) Entries() map[string]string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make(map[string]string, len(c.index.Entries))
	for k, v := range c.index.Entries {
		out[k] = v
	}
	return out
}
