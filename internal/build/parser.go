package build

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// Instruction represents a single parsed line from a Docksmithfile.
type Instruction struct {
	Line    int
	Command string
	Args    string
}

// validCommands is the set of recognised Docksmithfile instructions.
var validCommands = map[string]bool{
	"FROM":    true,
	"COPY":    true,
	"RUN":     true,
	"WORKDIR": true,
	"ENV":     true,
	"CMD":     true,
}

// Parse reads a Docksmithfile and returns the list of instructions.
// It rejects unrecognised instructions with a clear error and line number.
func Parse(path string) ([]Instruction, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open Docksmithfile: %w", err)
	}
	defer f.Close()

	var instructions []Instruction
	scanner := bufio.NewScanner(f)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip blank lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Handle line continuations
		for strings.HasSuffix(line, "\\") {
			if !scanner.Scan() {
				break
			}
			lineNum++
			line = strings.TrimSuffix(line, "\\") + " " + strings.TrimSpace(scanner.Text())
		}

		// Split into command and args
		parts := strings.SplitN(line, " ", 2)
		cmd := strings.ToUpper(parts[0])
		args := ""
		if len(parts) > 1 {
			args = strings.TrimSpace(parts[1])
		}

		if !validCommands[cmd] {
			return nil, fmt.Errorf("line %d: unrecognised instruction %q", lineNum, parts[0])
		}

		if cmd == "FROM" && len(instructions) > 0 {
			return nil, fmt.Errorf("line %d: FROM must be the first instruction", lineNum)
		}

		instructions = append(instructions, Instruction{
			Line:    lineNum,
			Command: cmd,
			Args:    args,
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan Docksmithfile: %w", err)
	}

	if len(instructions) == 0 {
		return nil, fmt.Errorf("Docksmithfile is empty")
	}

	if instructions[0].Command != "FROM" {
		return nil, fmt.Errorf("line %d: first instruction must be FROM, got %s", instructions[0].Line, instructions[0].Command)
	}

	return instructions, nil
}
