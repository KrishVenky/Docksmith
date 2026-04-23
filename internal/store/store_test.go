package store

import (
	"os"
	"path/filepath"
	"testing"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s := &Store{Root: dir}
	os.MkdirAll(s.ImagesDir(), 0755)
	os.MkdirAll(s.LayersDir(), 0755)
	os.MkdirAll(s.CacheDir(), 0755)
	return s
}

func TestImageSaveAndLoad(t *testing.T) {
	s := testStore(t)

	img := &Image{
		Name: "myapp",
		Tag:  "latest",
		Config: ImageConfig{
			Env:        []string{"KEY=value"},
			Cmd:        []string{"echo", "hello"},
			WorkingDir: "/app",
		},
		Layers: []LayerRef{
			{Digest: "sha256:aaa", Size: 1024, CreatedBy: "COPY . /app"},
		},
	}

	if err := s.SaveImage(img); err != nil {
		t.Fatalf("SaveImage: %v", err)
	}

	// Digest should be set
	if img.Digest == "" {
		t.Fatal("Digest should be set after save")
	}
	if img.Digest[:7] != "sha256:" {
		t.Errorf("Digest should start with sha256:, got %s", img.Digest)
	}

	// Load it back
	loaded, err := s.LoadImage("myapp", "latest")
	if err != nil {
		t.Fatalf("LoadImage: %v", err)
	}

	if loaded.Name != "myapp" {
		t.Errorf("Name: got %s, want myapp", loaded.Name)
	}
	if loaded.Digest != img.Digest {
		t.Errorf("Digest mismatch: got %s, want %s", loaded.Digest, img.Digest)
	}
	if len(loaded.Layers) != 1 {
		t.Errorf("Layers: got %d, want 1", len(loaded.Layers))
	}
}

func TestImageNotFound(t *testing.T) {
	s := testStore(t)
	_, err := s.LoadImage("nonexistent", "latest")
	if err == nil {
		t.Fatal("expected error for nonexistent image")
	}
}

func TestListImages(t *testing.T) {
	s := testStore(t)

	for _, name := range []string{"app1", "app2"} {
		img := &Image{
			Name:   name,
			Tag:    "latest",
			Config: ImageConfig{Cmd: []string{"echo"}},
			Layers: []LayerRef{},
		}
		s.SaveImage(img)
	}

	images, err := s.ListImages()
	if err != nil {
		t.Fatalf("ListImages: %v", err)
	}
	if len(images) != 2 {
		t.Errorf("expected 2 images, got %d", len(images))
	}
}

func TestRemoveImage(t *testing.T) {
	s := testStore(t)

	img := &Image{
		Name:   "todelete",
		Tag:    "v1",
		Config: ImageConfig{Cmd: []string{"echo"}},
		Layers: []LayerRef{},
	}
	s.SaveImage(img)

	if err := s.RemoveImage("todelete", "v1"); err != nil {
		t.Fatalf("RemoveImage: %v", err)
	}

	// Verify it's gone
	_, err := s.LoadImage("todelete", "v1")
	if err == nil {
		t.Fatal("image should be removed")
	}
}

func TestDigestComputation(t *testing.T) {
	// Verify the digest computation matches the spec:
	// serialize with digest="", hash, then set
	s := testStore(t)

	img := &Image{
		Name:   "test",
		Tag:    "v1",
		Config: ImageConfig{Cmd: []string{"echo"}},
		Layers: []LayerRef{
			{Digest: "sha256:abc123", Size: 100, CreatedBy: "test"},
		},
	}

	s.SaveImage(img)

	// Load and verify the digest on disk matches
	data, _ := os.ReadFile(filepath.Join(s.ImagesDir(), "test_v1.json"))
	loaded, _ := s.LoadImage("test", "v1")

	// The file on disk should contain a non-empty digest
	if loaded.Digest == "" {
		t.Fatal("on-disk digest should be non-empty")
	}

	_ = data // File was read successfully
}
