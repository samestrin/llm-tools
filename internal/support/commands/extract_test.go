package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractCommand(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "extract-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test file with various patterns
	testFile := filepath.Join(tmpDir, "content.txt")
	content := `# Test Document

Check out https://example.com/page and http://test.org/api for more info.

Contact us at support@example.com or sales@company.org

Files to edit:
- src/main.ts
- ./config/settings.json
- ~/projects/app.js

Template variables: {{user_name}}, {{api_key|default}}, {{count}}

## Tasks
- [x] Complete first task
- [ ] Pending second task
- [X] Another done task
- [ ] Last pending task

Server IPs: 192.168.1.1, 10.0.0.1, 255.255.255.0
`
	os.WriteFile(testFile, []byte(content), 0644)

	tests := []struct {
		name        string
		extractType string
		wantOutput  []string
		wantMissing []string
	}{
		{
			name:        "extract URLs",
			extractType: "urls",
			wantOutput:  []string{"https://example.com/page", "http://test.org/api"},
		},
		{
			name:        "extract emails",
			extractType: "emails",
			wantOutput:  []string{"support@example.com", "sales@company.org"},
		},
		{
			name:        "extract variables",
			extractType: "variables",
			wantOutput:  []string{"user_name", "api_key", "count"},
		},
		{
			name:        "extract todos",
			extractType: "todos",
			wantOutput:  []string{"[x] Complete first task", "[ ] Pending second task"},
		},
		{
			name:        "extract IPs",
			extractType: "ips",
			wantOutput:  []string{"192.168.1.1", "10.0.0.1", "255.255.255.0"},
		},
		{
			name:        "extract paths",
			extractType: "paths",
			wantOutput:  []string{"./config/settings.json", "~/projects/app.js"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newExtractCmd()
			var stdout, stderr bytes.Buffer
			cmd.SetOut(&stdout)
			cmd.SetErr(&stderr)
			cmd.SetArgs([]string{tt.extractType, testFile})

			err := cmd.Execute()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			output := stdout.String()
			for _, want := range tt.wantOutput {
				if !bytes.Contains([]byte(output), []byte(want)) {
					t.Errorf("output missing %q, got: %s", want, output)
				}
			}
		})
	}
}

func TestExtractUnique(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "extract-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "dups.txt")
	content := `email@test.com email@test.com email@test.com other@test.com`
	os.WriteFile(testFile, []byte(content), 0644)

	cmd := newExtractCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"emails", testFile, "--unique"})

	err = cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := stdout.String()
	// Count occurrences of email@test.com
	count := bytes.Count([]byte(output), []byte("email@test.com"))
	if count != 1 {
		t.Errorf("expected 1 occurrence with --unique, got %d", count)
	}
}

func TestExtractCount(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "extract-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "emails.txt")
	content := `a@b.com c@d.com e@f.com g@h.com`
	os.WriteFile(testFile, []byte(content), 0644)

	cmd := newExtractCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"emails", testFile, "--count"})

	err = cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := stdout.String()
	if !bytes.Contains([]byte(output), []byte("COUNT: 4")) {
		t.Errorf("expected COUNT: 4, got: %s", output)
	}
}

func TestExtractUnknownType(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "extract-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "file.txt")
	os.WriteFile(testFile, []byte("content"), 0644)

	cmd := newExtractCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"unknown_type", testFile})

	err = cmd.Execute()
	if err == nil {
		t.Error("expected error for unknown type")
	}
}

func TestExtractNoResults(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "extract-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "empty.txt")
	os.WriteFile(testFile, []byte("no patterns here"), 0644)

	cmd := newExtractCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"emails", testFile})

	err = cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !bytes.Contains([]byte(stderr.String()), []byte("No emails found")) {
		t.Errorf("expected 'No emails found' message")
	}
}
