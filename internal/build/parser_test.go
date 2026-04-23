package build

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseValid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Docksmithfile")
	content := `# This is a comment
FROM alpine:latest
WORKDIR /app
COPY . /app
RUN echo hello
ENV GREETING=hello
CMD ["echo", "hi"]
`
	os.WriteFile(path, []byte(content), 0644)

	instructions, err := Parse(path)
	if err != nil {
		t.Fatalf("Parse valid file: %v", err)
	}

	if len(instructions) != 6 {
		t.Fatalf("expected 6 instructions, got %d", len(instructions))
	}

	tests := []struct {
		cmd  string
		args string
	}{
		{"FROM", "alpine:latest"},
		{"WORKDIR", "/app"},
		{"COPY", ". /app"},
		{"RUN", "echo hello"},
		{"ENV", "GREETING=hello"},
		{"CMD", `["echo", "hi"]`},
	}

	for i, tt := range tests {
		if instructions[i].Command != tt.cmd {
			t.Errorf("instruction %d: cmd = %s, want %s", i, instructions[i].Command, tt.cmd)
		}
		if instructions[i].Args != tt.args {
			t.Errorf("instruction %d: args = %q, want %q", i, instructions[i].Args, tt.args)
		}
	}
}

func TestParseUnrecognisedInstruction(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Docksmithfile")
	content := `FROM alpine:latest
EXPOSE 8080
`
	os.WriteFile(path, []byte(content), 0644)

	_, err := Parse(path)
	if err == nil {
		t.Fatal("expected error for unrecognised instruction")
	}

	// Should mention line number
	if !contains(err.Error(), "line 2") {
		t.Errorf("error should mention line number: %v", err)
	}
	if !contains(err.Error(), "EXPOSE") {
		t.Errorf("error should mention the bad instruction: %v", err)
	}
}

func TestParseFROMMustBeFirst(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Docksmithfile")
	content := `RUN echo hello
FROM alpine:latest
`
	os.WriteFile(path, []byte(content), 0644)

	_, err := Parse(path)
	if err == nil {
		t.Fatal("expected error when FROM is not first")
	}
}

func TestParseEmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Docksmithfile")
	os.WriteFile(path, []byte(""), 0644)

	_, err := Parse(path)
	if err == nil {
		t.Fatal("expected error for empty file")
	}
}

func TestParseCommentsAndBlanks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Docksmithfile")
	content := `
# Comment 1

FROM alpine:latest

# Another comment
RUN echo hi

`
	os.WriteFile(path, []byte(content), 0644)

	instructions, err := Parse(path)
	if err != nil {
		t.Fatalf("Parse with comments: %v", err)
	}

	if len(instructions) != 2 {
		t.Errorf("expected 2 instructions, got %d", len(instructions))
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsSubstring(s, sub))
}

func containsSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
