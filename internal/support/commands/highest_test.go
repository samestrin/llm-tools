package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHighestCommand_Plans(t *testing.T) {
	// Create temp directory with plan-like structure
	tmpDir := t.TempDir()
	plansDir := filepath.Join(tmpDir, "plans")
	os.MkdirAll(plansDir, 0755)

	// Create plan directories
	os.MkdirAll(filepath.Join(plansDir, "1.0_first_plan"), 0755)
	os.MkdirAll(filepath.Join(plansDir, "2.0_second_plan"), 0755)
	os.MkdirAll(filepath.Join(plansDir, "10.0_tenth_plan"), 0755)
	os.MkdirAll(filepath.Join(plansDir, "3.0_third_plan"), 0755)

	// Run command
	cmd := newHighestCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--path", plansDir, "--type", "dir"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "HIGHEST: 10.0") {
		t.Errorf("expected HIGHEST: 10.0, got: %s", output)
	}
	if !strings.Contains(output, "NEXT: 11.0") {
		t.Errorf("expected NEXT: 11.0, got: %s", output)
	}
	if !strings.Contains(output, "COUNT: 4") {
		t.Errorf("expected COUNT: 4, got: %s", output)
	}
}

func TestHighestCommand_UserStories(t *testing.T) {
	// Create temp directory with user stories structure
	tmpDir := t.TempDir()
	storiesDir := filepath.Join(tmpDir, "user-stories")
	os.MkdirAll(storiesDir, 0755)

	// Create story files
	os.WriteFile(filepath.Join(storiesDir, "01-first-story.md"), []byte("test"), 0644)
	os.WriteFile(filepath.Join(storiesDir, "02-second-story.md"), []byte("test"), 0644)
	os.WriteFile(filepath.Join(storiesDir, "05-fifth-story.md"), []byte("test"), 0644)

	// Run command
	cmd := newHighestCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--path", storiesDir, "--type", "file"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "HIGHEST: 05") {
		t.Errorf("expected HIGHEST: 05, got: %s", output)
	}
	if !strings.Contains(output, "NEXT: 06") {
		t.Errorf("expected NEXT: 06, got: %s", output)
	}
}

func TestHighestCommand_AcceptanceCriteria(t *testing.T) {
	// Create temp directory with AC structure
	tmpDir := t.TempDir()
	acDir := filepath.Join(tmpDir, "acceptance-criteria")
	os.MkdirAll(acDir, 0755)

	// Create AC files for multiple stories
	os.WriteFile(filepath.Join(acDir, "01-01-first-ac.md"), []byte("test"), 0644)
	os.WriteFile(filepath.Join(acDir, "01-02-second-ac.md"), []byte("test"), 0644)
	os.WriteFile(filepath.Join(acDir, "02-01-story2-first.md"), []byte("test"), 0644)
	os.WriteFile(filepath.Join(acDir, "02-03-story2-third.md"), []byte("test"), 0644)

	// Run command - highest overall
	cmd := newHighestCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--path", acDir, "--type", "file"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "HIGHEST: 02-03") {
		t.Errorf("expected HIGHEST: 02-03, got: %s", output)
	}
}

func TestHighestCommand_AcceptanceCriteriaWithPrefix(t *testing.T) {
	// Create temp directory with AC structure
	tmpDir := t.TempDir()
	acDir := filepath.Join(tmpDir, "acceptance-criteria")
	os.MkdirAll(acDir, 0755)

	// Create AC files for multiple stories
	os.WriteFile(filepath.Join(acDir, "01-01-first-ac.md"), []byte("test"), 0644)
	os.WriteFile(filepath.Join(acDir, "01-02-second-ac.md"), []byte("test"), 0644)
	os.WriteFile(filepath.Join(acDir, "01-04-fourth-ac.md"), []byte("test"), 0644)
	os.WriteFile(filepath.Join(acDir, "02-01-story2-first.md"), []byte("test"), 0644)
	os.WriteFile(filepath.Join(acDir, "02-03-story2-third.md"), []byte("test"), 0644)

	// Run command - highest within story 01
	cmd := newHighestCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--path", acDir, "--type", "file", "--prefix", "01-"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "HIGHEST: 04") {
		t.Errorf("expected HIGHEST: 04, got: %s", output)
	}
	if !strings.Contains(output, "NEXT: 05") {
		t.Errorf("expected NEXT: 05, got: %s", output)
	}
	if !strings.Contains(output, "COUNT: 3") {
		t.Errorf("expected COUNT: 3 (files with prefix 01-), got: %s", output)
	}
}

func TestHighestCommand_Tasks(t *testing.T) {
	// Create temp directory with tasks structure
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, "tasks")
	os.MkdirAll(tasksDir, 0755)

	// Create task files (both formats)
	os.WriteFile(filepath.Join(tasksDir, "task-01-setup.md"), []byte("test"), 0644)
	os.WriteFile(filepath.Join(tasksDir, "task-02-implement.md"), []byte("test"), 0644)
	os.WriteFile(filepath.Join(tasksDir, "task-05-cleanup.md"), []byte("test"), 0644)

	// Run command
	cmd := newHighestCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--path", tasksDir, "--type", "file"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "HIGHEST: 05") {
		t.Errorf("expected HIGHEST: 05, got: %s", output)
	}
}

func TestHighestCommand_TechnicalDebt(t *testing.T) {
	// Create temp directory with TD structure
	tmpDir := t.TempDir()
	tdDir := filepath.Join(tmpDir, "technical-debt")
	os.MkdirAll(tdDir, 0755)

	// Create TD files (mixed case and separators)
	os.WriteFile(filepath.Join(tdDir, "td-01-cleanup.md"), []byte("test"), 0644)
	os.WriteFile(filepath.Join(tdDir, "TD-15_refactor.md"), []byte("test"), 0644)
	os.WriteFile(filepath.Join(tdDir, "td-22-upgrade.md"), []byte("test"), 0644)

	// Run command
	cmd := newHighestCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--path", tdDir, "--type", "file"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "HIGHEST: 22") {
		t.Errorf("expected HIGHEST: 22, got: %s", output)
	}
}

func TestHighestCommand_EmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	cmd := newHighestCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--path", tmpDir})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "HIGHEST: ") && !strings.Contains(output, "COUNT: 0") {
		t.Errorf("expected empty result, got: %s", output)
	}
}

func TestHighestCommand_JSONOutput(t *testing.T) {
	tmpDir := t.TempDir()
	plansDir := filepath.Join(tmpDir, "plans")
	os.MkdirAll(plansDir, 0755)
	os.MkdirAll(filepath.Join(plansDir, "1.0_first"), 0755)
	os.MkdirAll(filepath.Join(plansDir, "2.0_second"), 0755)

	cmd := newHighestCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--path", plansDir, "--type", "dir", "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, `"highest": "2.0"`) {
		t.Errorf("expected JSON with highest: 2.0, got: %s", output)
	}
	if !strings.Contains(output, `"next": "3.0"`) {
		t.Errorf("expected JSON with next: 3.0, got: %s", output)
	}
}

func TestHighestCommand_Sprints(t *testing.T) {
	// Create temp directory with sprint structure (nested in active/completed)
	tmpDir := t.TempDir()
	sprintsDir := filepath.Join(tmpDir, "sprints", "active")
	os.MkdirAll(sprintsDir, 0755)

	// Create sprint directories
	os.MkdirAll(filepath.Join(sprintsDir, "114.0_feature-one"), 0755)
	os.MkdirAll(filepath.Join(sprintsDir, "115.0_feature-two"), 0755)

	// Run command
	cmd := newHighestCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--path", sprintsDir, "--type", "dir"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "HIGHEST: 115.0") {
		t.Errorf("expected HIGHEST: 115.0, got: %s", output)
	}
	if !strings.Contains(output, "NEXT: 116.0") {
		t.Errorf("expected NEXT: 116.0, got: %s", output)
	}
}

func TestHighestCommand_PlansWithSubdirectories(t *testing.T) {
	// Create temp directory with plans structure (active/pending/completed like sprints)
	tmpDir := t.TempDir()

	// Create all three subdirectories
	activeDir := filepath.Join(tmpDir, "plans", "active")
	pendingDir := filepath.Join(tmpDir, "plans", "pending")
	completedDir := filepath.Join(tmpDir, "plans", "completed")
	os.MkdirAll(activeDir, 0755)
	os.MkdirAll(pendingDir, 0755)
	os.MkdirAll(completedDir, 0755)

	// Create plan directories in active
	os.MkdirAll(filepath.Join(activeDir, "115.0_current-feature"), 0755)
	os.MkdirAll(filepath.Join(activeDir, "116.0_another-feature"), 0755)

	// Create plan directories in pending
	os.MkdirAll(filepath.Join(pendingDir, "117.0_upcoming-feature"), 0755)

	// Create plan directories in completed
	os.MkdirAll(filepath.Join(completedDir, "110.0_old-feature"), 0755)
	os.MkdirAll(filepath.Join(completedDir, "114.0_recent-feature"), 0755)

	// Test active directory
	t.Run("active", func(t *testing.T) {
		cmd := newHighestCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetArgs([]string{"--path", activeDir, "--type", "dir"})

		err := cmd.Execute()
		if err != nil {
			t.Fatalf("command failed: %v", err)
		}

		output := buf.String()
		if !strings.Contains(output, "HIGHEST: 116.0") {
			t.Errorf("expected HIGHEST: 116.0, got: %s", output)
		}
		if !strings.Contains(output, "COUNT: 2") {
			t.Errorf("expected COUNT: 2, got: %s", output)
		}
	})

	// Test pending directory
	t.Run("pending", func(t *testing.T) {
		cmd := newHighestCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetArgs([]string{"--path", pendingDir, "--type", "dir"})

		err := cmd.Execute()
		if err != nil {
			t.Fatalf("command failed: %v", err)
		}

		output := buf.String()
		if !strings.Contains(output, "HIGHEST: 117.0") {
			t.Errorf("expected HIGHEST: 117.0, got: %s", output)
		}
	})

	// Test completed directory
	t.Run("completed", func(t *testing.T) {
		cmd := newHighestCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetArgs([]string{"--path", completedDir, "--type", "dir"})

		err := cmd.Execute()
		if err != nil {
			t.Fatalf("command failed: %v", err)
		}

		output := buf.String()
		if !strings.Contains(output, "HIGHEST: 114.0") {
			t.Errorf("expected HIGHEST: 114.0, got: %s", output)
		}
		if !strings.Contains(output, "COUNT: 2") {
			t.Errorf("expected COUNT: 2, got: %s", output)
		}
	})
}

func TestBuildVersionInfo(t *testing.T) {
	tests := []struct {
		name        string
		matches     []string
		wantVersion string
		wantSortKey float64
	}{
		{
			name:        "single group",
			matches:     []string{"01-", "01"},
			wantVersion: "01",
			wantSortKey: 1,
		},
		{
			name:        "decimal version",
			matches:     []string{"115.0_", "115", "0"},
			wantVersion: "115.0",
			wantSortKey: 115.0,
		},
		{
			name:        "compound version with hyphen",
			matches:     []string{"01-02-", "01", "02"},
			wantVersion: "01-02",
			wantSortKey: 1002,
		},
		{
			name:        "compound version with underscore",
			matches:     []string{"01_02_", "01", "02"},
			wantVersion: "01-02",
			wantSortKey: 1002,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			version, sortKey := buildVersionInfo(tt.matches)
			if version != tt.wantVersion {
				t.Errorf("version = %s, want %s", version, tt.wantVersion)
			}
			if sortKey != tt.wantSortKey {
				t.Errorf("sortKey = %f, want %f", sortKey, tt.wantSortKey)
			}
		})
	}
}

func TestCalculateNext(t *testing.T) {
	tests := []struct {
		version string
		prefix  string
		want    string
	}{
		{"115.0", "", "116.0"},
		{"1.0", "", "2.0"},
		{"01", "", "02"},
		{"05", "", "06"},
		{"03", "01-", "04"},
		{"01-03", "", "01-04"},
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			got := calculateNext(tt.version, tt.prefix)
			if got != tt.want {
				t.Errorf("calculateNext(%s, %s) = %s, want %s", tt.version, tt.prefix, got, tt.want)
			}
		})
	}
}
