package main

import (
	"os"
	"os/exec"
	"testing"
)

func TestBinaryBuilds(t *testing.T) {
	// This test ensures the binary compiles successfully
	cmd := exec.Command("go", "build", "-o", "/dev/null", ".")
	cmd.Dir = "."
	if err := cmd.Run(); err != nil {
		t.Fatalf("Binary failed to build: %v", err)
	}
}

func TestHelpFlag(t *testing.T) {
	// Skip if binary not built
	if _, err := os.Stat("../../build/llm-filesystem"); os.IsNotExist(err) {
		t.Skip("Binary not built, skipping integration test")
	}

	cmd := exec.Command("../../build/llm-filesystem", "--help")
	output, err := cmd.CombinedOutput()
	if err != nil {
		// --help should exit with code 0
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() != 0 {
			t.Fatalf("--help failed: %v\nOutput: %s", err, output)
		}
	}

	if len(output) == 0 {
		t.Error("--help produced no output")
	}
}
