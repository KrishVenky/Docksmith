package store

import (
	"testing"

	"docksmith/internal/util"
)

func TestWriteAndReadLayer(t *testing.T) {
	s := testStore(t)

	content := []byte("hello layer content")
	expectedDigest := util.HashBytes(content)

	digest, size, err := s.WriteLayer(content)
	if err != nil {
		t.Fatalf("WriteLayer: %v", err)
	}

	if digest != expectedDigest {
		t.Errorf("digest: got %s, want %s", digest, expectedDigest)
	}
	if size != int64(len(content)) {
		t.Errorf("size: got %d, want %d", size, len(content))
	}

	// Read it back
	data, err := s.ReadLayer(digest)
	if err != nil {
		t.Fatalf("ReadLayer: %v", err)
	}
	if string(data) != string(content) {
		t.Errorf("content mismatch")
	}
}

func TestLayerExists(t *testing.T) {
	s := testStore(t)

	content := []byte("test")
	digest, _, _ := s.WriteLayer(content)

	if !s.LayerExists(digest) {
		t.Error("LayerExists should return true")
	}
	if s.LayerExists("sha256:nonexistent") {
		t.Error("LayerExists should return false for nonexistent")
	}
}

func TestRemoveLayer(t *testing.T) {
	s := testStore(t)

	content := []byte("removable")
	digest, _, _ := s.WriteLayer(content)

	if err := s.RemoveLayer(digest); err != nil {
		t.Fatalf("RemoveLayer: %v", err)
	}

	if s.LayerExists(digest) {
		t.Error("layer should not exist after removal")
	}
}
