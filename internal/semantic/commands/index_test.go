package commands

import (
	"bytes"
	"testing"

	"github.com/samestrin/llm-tools/internal/semantic"
)

func TestIndexCmd_Help(t *testing.T) {
	cmd := indexCmd()

	if cmd.Use != "index [path]" {
		t.Errorf("Use = %q, want 'index [path]'", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("Short description should not be empty")
	}
}

func TestIndexCmd_Flags(t *testing.T) {
	cmd := indexCmd()

	flags := []struct {
		name     string
		shortcut string
	}{
		{"include", "i"},
		{"exclude", "e"},
		{"force", "f"},
		{"json", ""},
	}

	for _, f := range flags {
		flag := cmd.Flags().Lookup(f.name)
		if flag == nil {
			t.Errorf("Flag --%s not found", f.name)
			continue
		}
		if f.shortcut != "" {
			shortFlag := cmd.Flags().ShorthandLookup(f.shortcut)
			if shortFlag == nil {
				t.Errorf("Shorthand -%s not found for --%s", f.shortcut, f.name)
			}
		}
	}
}

func TestIndexCmd_DefaultExcludes(t *testing.T) {
	cmd := indexCmd()

	excludeFlag := cmd.Flags().Lookup("exclude")
	if excludeFlag == nil {
		t.Fatal("exclude flag not found")
	}

	// Default should include common directories to exclude
	defVal := excludeFlag.DefValue
	expectedExcludes := []string{"vendor", "node_modules", ".git"}

	for _, exc := range expectedExcludes {
		if !bytes.Contains([]byte(defVal), []byte(exc)) {
			t.Errorf("Default excludes should contain %q", exc)
		}
	}
}

func TestIndexCmd_OptionalPath(t *testing.T) {
	cmd := indexCmd()

	// Should accept 0 or 1 arguments
	err := cmd.Args(cmd, []string{})
	if err != nil {
		t.Errorf("Should accept no arguments: %v", err)
	}

	err = cmd.Args(cmd, []string{"./some/path"})
	if err != nil {
		t.Errorf("Should accept path argument: %v", err)
	}

	err = cmd.Args(cmd, []string{"path1", "path2"})
	if err == nil {
		t.Error("Should reject multiple paths")
	}
}

func TestIndexStatusCmd_Help(t *testing.T) {
	cmd := indexStatusCmd()

	if cmd.Use != "index-status" {
		t.Errorf("Use = %q, want 'index-status'", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("Short description should not be empty")
	}

	// Should have json flag
	jsonFlag := cmd.Flags().Lookup("json")
	if jsonFlag == nil {
		t.Error("index-status should have --json flag")
	}
}

func TestIndexUpdateCmd_Help(t *testing.T) {
	cmd := indexUpdateCmd()

	if cmd.Use != "index-update [path]" {
		t.Errorf("Use = %q, want 'index-update [path]'", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("Short description should not be empty")
	}
}

func TestIndexUpdateCmd_Flags(t *testing.T) {
	cmd := indexUpdateCmd()

	flags := []string{"include", "exclude", "json"}

	for _, name := range flags {
		flag := cmd.Flags().Lookup(name)
		if flag == nil {
			t.Errorf("Flag --%s not found", name)
		}
	}
}

func TestRootCmd_HasIndexCommands(t *testing.T) {
	root := RootCmd()

	expectedCommands := []string{"index", "index-status", "index-update"}

	for _, name := range expectedCommands {
		var found bool
		for _, cmd := range root.Commands() {
			if cmd.Name() == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Root command should have '%s' subcommand", name)
		}
	}
}

func TestIndexCmd_HelpOutput(t *testing.T) {
	root := RootCmd()
	output, err := executeCommand(root, "index", "--help")

	if err != nil {
		t.Fatalf("Help command failed: %v", err)
	}

	expectedStrings := []string{
		"index",
		"--include",
		"--exclude",
		"--force",
		"--json",
	}

	for _, s := range expectedStrings {
		if !bytes.Contains([]byte(output), []byte(s)) {
			t.Errorf("Help output should contain %q", s)
		}
	}
}

func TestIndexStatusCmd_HelpOutput(t *testing.T) {
	root := RootCmd()
	output, err := executeCommand(root, "index-status", "--help")

	if err != nil {
		t.Fatalf("Help command failed: %v", err)
	}

	if !bytes.Contains([]byte(output), []byte("index-status")) {
		t.Error("Help output should contain 'index-status'")
	}
}

func TestIndexUpdateCmd_HelpOutput(t *testing.T) {
	root := RootCmd()
	output, err := executeCommand(root, "index-update", "--help")

	if err != nil {
		t.Fatalf("Help command failed: %v", err)
	}

	expectedStrings := []string{
		"index-update",
		"--include",
		"--exclude",
	}

	for _, s := range expectedStrings {
		if !bytes.Contains([]byte(output), []byte(s)) {
			t.Errorf("Help output should contain %q", s)
		}
	}
}

func TestRegisterAllChunkers(t *testing.T) {
	// This test verifies that RegisterAllChunkers registers chunkers for all expected extensions.
	// Previously, index.go and index_update.go had duplicated registration code.
	// Now they share RegisterAllChunkers to ensure consistency.
	factory := semantic.NewChunkerFactory()
	RegisterAllChunkers(factory)

	exts := factory.SupportedExtensions()
	extSet := make(map[string]bool)
	for _, ext := range exts {
		extSet[ext] = true
	}

	// Required extensions from each chunker type
	requiredExtensions := []struct {
		ext   string
		descr string
	}{
		{"go", "GoChunker"},
		{"js", "JSChunker"},
		{"ts", "JSChunker"},
		{"py", "PythonChunker"},
		{"php", "PHPChunker"},
		{"rs", "RustChunker"},
		{"md", "MarkdownChunker"},
		{"markdown", "MarkdownChunker"},
		{"html", "HTMLChunker"},
		{"htm", "HTMLChunker"},
		{"txt", "GenericChunker"},
		{"yaml", "GenericChunker"},
	}

	for _, req := range requiredExtensions {
		if !extSet[req.ext] {
			t.Errorf("RegisterAllChunkers should register %q extension (from %s)", req.ext, req.descr)
		}
	}

	// Verify we have a reasonable number of extensions
	if len(exts) < 15 {
		t.Errorf("Expected at least 15 registered extensions, got %d", len(exts))
	}
}

func TestRegisterAllChunkers_ChunkerTypesCorrect(t *testing.T) {
	factory := semantic.NewChunkerFactory()
	RegisterAllChunkers(factory)

	// Verify correct chunker type for key extensions
	tests := []struct {
		ext      string
		checkFn  func(semantic.Chunker) bool
		wantType string
	}{
		{"go", func(c semantic.Chunker) bool { _, ok := c.(*semantic.GoChunker); return ok }, "GoChunker"},
		{"md", func(c semantic.Chunker) bool { _, ok := c.(*semantic.MarkdownChunker); return ok }, "MarkdownChunker"},
		{"html", func(c semantic.Chunker) bool { _, ok := c.(*semantic.HTMLChunker); return ok }, "HTMLChunker"},
		{"txt", func(c semantic.Chunker) bool { _, ok := c.(*semantic.GenericChunker); return ok }, "GenericChunker"},
	}

	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			chunker, ok := factory.GetChunker(tt.ext)
			if !ok {
				t.Fatalf("Extension %q not registered", tt.ext)
			}
			if !tt.checkFn(chunker) {
				t.Errorf("Extension %q should be registered with %s", tt.ext, tt.wantType)
			}
		})
	}
}
