package util

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHashBytes(t *testing.T) {
	// Known SHA-256 of empty string
	got := HashBytes([]byte(""))
	want := "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	if got != want {
		t.Errorf("HashBytes empty: got %s, want %s", got, want)
	}

	// Known SHA-256 of "hello"
	got = HashBytes([]byte("hello"))
	want = "sha256:2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	if got != want {
		t.Errorf("HashBytes hello: got %s, want %s", got, want)
	}
}

func TestHashFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	os.WriteFile(path, []byte("hello"), 0644)

	got, err := HashFile(path)
	if err != nil {
		t.Fatalf("HashFile: %v", err)
	}
	want := "sha256:2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	if got != want {
		t.Errorf("HashFile: got %s, want %s", got, want)
	}
}

func TestHashFileNotFound(t *testing.T) {
	_, err := HashFile("/nonexistent/file")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}
