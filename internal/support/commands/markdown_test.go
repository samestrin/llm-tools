package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMarkdownHeadersCommand(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "test.md")
	os.WriteFile(mdFile, []byte(`# Heading 1
Some content here.
## Heading 2
More content.
### Heading 3
Even more content.
## Another H2
`), 0644)

	tests := []struct {
		name     string
		args     []string
		expected []string
		hasError bool
	}{
		{
			name:     "extract all headers",
			args:     []string{"headers", mdFile},
			expected: []string{"# Heading 1", "## Heading 2", "### Heading 3", "## Another H2"},
		},
		{
			name:     "extract level 2 only",
			args:     []string{"headers", mdFile, "--level", "2"},
			expected: []string{"## Heading 2", "## Another H2"},
		},
		{
			name:     "plain output",
			args:     []string{"headers", mdFile, "--plain"},
			expected: []string{"Heading 1", "Heading 2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newMarkdownCmd()
			buf := new(bytes.Buffer)
			errBuf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetErr(errBuf)
			cmd.SetArgs(tt.args)

			err := cmd.Execute()

			if tt.hasError {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			output := buf.String()
			for _, exp := range tt.expected {
				if !strings.Contains(output, exp) {
					t.Errorf("output %q should contain %q", output, exp)
				}
			}
		})
	}
}

func TestMarkdownTasksCommand(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "tasks.md")
	os.WriteFile(mdFile, []byte(`# Task List
- [x] Task 1 completed
- [ ] Task 2 pending
- [x] Task 3 completed
- [ ] Task 4 pending
`), 0644)

	tests := []struct {
		name     string
		args     []string
		expected []string
		hasError bool
	}{
		{
			name:     "list all tasks",
			args:     []string{"tasks", mdFile},
			expected: []string{"Task 1", "Task 2", "TOTAL_TASKS: 4"},
		},
		{
			name:     "summary only",
			args:     []string{"tasks", mdFile, "--summary"},
			expected: []string{"TOTAL_TASKS: 4", "COMPLETED: 2", "INCOMPLETE: 2", "50.0%"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newMarkdownCmd()
			buf := new(bytes.Buffer)
			errBuf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetErr(errBuf)
			cmd.SetArgs(tt.args)

			err := cmd.Execute()

			if tt.hasError {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			output := buf.String()
			for _, exp := range tt.expected {
				if !strings.Contains(output, exp) {
					t.Errorf("output %q should contain %q", output, exp)
				}
			}
		})
	}
}

func TestMarkdownSectionCommand(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "sections.md")
	os.WriteFile(mdFile, []byte(`# Introduction
This is the intro.

## Getting Started
Here's how to get started.

### Prerequisites
You need these things.

## Installation
Install the package.
`), 0644)

	tests := []struct {
		name     string
		args     []string
		expected []string
		hasError bool
	}{
		{
			name:     "extract section by title",
			args:     []string{"section", mdFile, "Getting Started"},
			expected: []string{"how to get started"},
		},
		{
			name:     "section not found",
			args:     []string{"section", mdFile, "Nonexistent"},
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newMarkdownCmd()
			buf := new(bytes.Buffer)
			errBuf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetErr(errBuf)
			cmd.SetArgs(tt.args)

			err := cmd.Execute()

			if tt.hasError {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			output := buf.String()
			for _, exp := range tt.expected {
				if !strings.Contains(output, exp) {
					t.Errorf("output %q should contain %q", output, exp)
				}
			}
		})
	}
}

func TestMarkdownFrontmatterCommand(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "frontmatter.md")
	os.WriteFile(mdFile, []byte(`---
title: My Document
author: Test Author
date: 2025-01-01
---
# Content
Here is the content.
`), 0644)

	noFrontmatter := filepath.Join(tmpDir, "no-fm.md")
	os.WriteFile(noFrontmatter, []byte(`# Just content
No frontmatter here.
`), 0644)

	tests := []struct {
		name     string
		args     []string
		expected []string
		hasError bool
	}{
		{
			name:     "extract frontmatter",
			args:     []string{"frontmatter", mdFile},
			expected: []string{"title:", "author:"},
		},
		{
			name:     "frontmatter as JSON",
			args:     []string{"frontmatter", mdFile, "--json"},
			expected: []string{`"title"`, `"author"`},
		},
		{
			name:     "no frontmatter",
			args:     []string{"frontmatter", noFrontmatter},
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newMarkdownCmd()
			buf := new(bytes.Buffer)
			errBuf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetErr(errBuf)
			cmd.SetArgs(tt.args)

			err := cmd.Execute()

			if tt.hasError {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			output := buf.String()
			for _, exp := range tt.expected {
				if !strings.Contains(output, exp) {
					t.Errorf("output %q should contain %q", output, exp)
				}
			}
		})
	}
}

func TestMarkdownCodeblocksCommand(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "codeblocks.md")
	os.WriteFile(mdFile, []byte("# Code Examples\n\n```go\nfunc main() {}\n```\n\n```python\nprint('hello')\n```\n\n```go\nfunc test() {}\n```\n"), 0644)

	tests := []struct {
		name     string
		args     []string
		expected []string
		hasError bool
	}{
		{
			name:     "extract all codeblocks",
			args:     []string{"codeblocks", mdFile},
			expected: []string{"func main", "print('hello')", "func test"},
		},
		{
			name:     "filter by language",
			args:     []string{"codeblocks", mdFile, "--language", "go"},
			expected: []string{"func main", "func test"},
		},
		{
			name:     "list only",
			args:     []string{"codeblocks", mdFile, "--list"},
			expected: []string{"Block 1: go", "Block 2: python", "Block 3: go"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newMarkdownCmd()
			buf := new(bytes.Buffer)
			errBuf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetErr(errBuf)
			cmd.SetArgs(tt.args)

			err := cmd.Execute()

			if tt.hasError {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			output := buf.String()
			for _, exp := range tt.expected {
				if !strings.Contains(output, exp) {
					t.Errorf("output %q should contain %q", output, exp)
				}
			}
		})
	}
}
