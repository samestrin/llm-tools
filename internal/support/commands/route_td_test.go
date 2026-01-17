package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// TestRouteTDBasicRouting tests basic threshold routing
func TestRouteTDBasicRouting(t *testing.T) {
	tests := []struct {
		name              string
		input             string
		expectedQuickWins int
		expectedBacklog   int
		expectedTDFiles   int
	}{
		{
			name: "quick wins only (< 30 min)",
			input: `{
				"rows": [
					{"ID": "TD-001", "CATEGORY": "performance", "EST_MINUTES": 15},
					{"ID": "TD-002", "CATEGORY": "cleanup", "EST_MINUTES": 10},
					{"ID": "TD-003", "CATEGORY": "docs", "EST_MINUTES": 5}
				]
			}`,
			expectedQuickWins: 3,
			expectedBacklog:   0,
			expectedTDFiles:   0,
		},
		{
			name: "backlog only (30-2879 min)",
			input: `{
				"rows": [
					{"ID": "TD-001", "CATEGORY": "refactoring", "EST_MINUTES": 120},
					{"ID": "TD-002", "CATEGORY": "security", "EST_MINUTES": 480}
				]
			}`,
			expectedQuickWins: 0,
			expectedBacklog:   2,
			expectedTDFiles:   0,
		},
		{
			name: "td files only (>= 2880 min)",
			input: `{
				"rows": [
					{"ID": "TD-001", "CATEGORY": "architecture", "EST_MINUTES": 2880},
					{"ID": "TD-002", "CATEGORY": "migration", "EST_MINUTES": 4320}
				]
			}`,
			expectedQuickWins: 0,
			expectedBacklog:   0,
			expectedTDFiles:   2,
		},
		{
			name: "mixed routing",
			input: `{
				"rows": [
					{"ID": "TD-001", "CATEGORY": "performance", "EST_MINUTES": 15},
					{"ID": "TD-002", "CATEGORY": "refactoring", "EST_MINUTES": 120},
					{"ID": "TD-003", "CATEGORY": "architecture", "EST_MINUTES": 2880}
				]
			}`,
			expectedQuickWins: 1,
			expectedBacklog:   1,
			expectedTDFiles:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newRouteTDCmd()
			buf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetArgs([]string{"--content", tt.input, "--json"})

			err := cmd.Execute()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			var result RouteTDResult
			if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
				t.Fatalf("failed to parse JSON output: %v\nOutput: %s", err, buf.String())
			}

			if len(result.QuickWins) != tt.expectedQuickWins {
				t.Errorf("quick_wins count = %d, want %d", len(result.QuickWins), tt.expectedQuickWins)
			}

			if len(result.Backlog) != tt.expectedBacklog {
				t.Errorf("backlog count = %d, want %d", len(result.Backlog), tt.expectedBacklog)
			}

			if len(result.TDFiles) != tt.expectedTDFiles {
				t.Errorf("td_files count = %d, want %d", len(result.TDFiles), tt.expectedTDFiles)
			}
		})
	}
}

// TestRouteTDBoundaryValues tests exact threshold boundaries
func TestRouteTDBoundaryValues(t *testing.T) {
	tests := []struct {
		name        string
		estMinutes  int
		destination string // "quick_wins", "backlog", or "td_files"
	}{
		{"EST_MINUTES=0 -> quick_wins", 0, "quick_wins"},
		{"EST_MINUTES=29 -> quick_wins", 29, "quick_wins"},
		{"EST_MINUTES=30 -> backlog (boundary)", 30, "backlog"},
		{"EST_MINUTES=31 -> backlog", 31, "backlog"},
		{"EST_MINUTES=2879 -> backlog", 2879, "backlog"},
		{"EST_MINUTES=2880 -> td_files (boundary)", 2880, "td_files"},
		{"EST_MINUTES=2881 -> td_files", 2881, "td_files"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use proper JSON encoding
			inputJSON := map[string]interface{}{
				"rows": []map[string]interface{}{
					{"ID": "TD-001", "CATEGORY": "test", "EST_MINUTES": tt.estMinutes},
				},
			}
			inputBytes, _ := json.Marshal(inputJSON)

			cmd := newRouteTDCmd()
			buf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetArgs([]string{"--content", string(inputBytes), "--json"})

			err := cmd.Execute()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			var result RouteTDResult
			if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
				t.Fatalf("failed to parse JSON output: %v", err)
			}

			switch tt.destination {
			case "quick_wins":
				if len(result.QuickWins) != 1 {
					t.Errorf("expected 1 in quick_wins, got %d", len(result.QuickWins))
				}
			case "backlog":
				if len(result.Backlog) != 1 {
					t.Errorf("expected 1 in backlog, got %d", len(result.Backlog))
				}
			case "td_files":
				if len(result.TDFiles) != 1 {
					t.Errorf("expected 1 in td_files, got %d", len(result.TDFiles))
				}
			}
		})
	}
}

// TestRouteTDZeroDataLoss tests that no issues are dropped
func TestRouteTDZeroDataLoss(t *testing.T) {
	// Create input with 45 issues (like the original bug)
	rows := make([]map[string]interface{}, 45)
	for i := 0; i < 45; i++ {
		estMinutes := (i % 3) * 1500 // 0, 1500, 3000 cycle
		rows[i] = map[string]interface{}{
			"ID":          "TD-" + string(rune('A'+i%26)),
			"CATEGORY":    "test",
			"EST_MINUTES": estMinutes,
		}
	}
	input := map[string]interface{}{"rows": rows}
	inputBytes, _ := json.Marshal(input)

	cmd := newRouteTDCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--content", string(inputBytes), "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result RouteTDResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	totalRouted := len(result.QuickWins) + len(result.Backlog) + len(result.TDFiles)
	if totalRouted != 45 {
		t.Errorf("DATA LOSS: input=45, routed=%d (quick_wins=%d, backlog=%d, td_files=%d)",
			totalRouted, len(result.QuickWins), len(result.Backlog), len(result.TDFiles))
	}

	if result.Summary.TotalInput != 45 {
		t.Errorf("summary.total_input = %d, want 45", result.Summary.TotalInput)
	}

	if result.Summary.TotalRouted != 45 {
		t.Errorf("summary.total_routed = %d, want 45", result.Summary.TotalRouted)
	}
}

// TestRouteTDStringEstMinutes tests EST_MINUTES as string (from parsed pipe-delimited)
func TestRouteTDStringEstMinutes(t *testing.T) {
	input := `{
		"rows": [
			{"ID": "TD-001", "CATEGORY": "test", "EST_MINUTES": "15"},
			{"ID": "TD-002", "CATEGORY": "test", "EST_MINUTES": "120"},
			{"ID": "TD-003", "CATEGORY": "test", "EST_MINUTES": "3000"}
		]
	}`

	cmd := newRouteTDCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--content", input, "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result RouteTDResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if len(result.QuickWins) != 1 {
		t.Errorf("quick_wins = %d, want 1 (string '15' should parse)", len(result.QuickWins))
	}

	if len(result.Backlog) != 1 {
		t.Errorf("backlog = %d, want 1 (string '120' should parse)", len(result.Backlog))
	}

	if len(result.TDFiles) != 1 {
		t.Errorf("td_files = %d, want 1 (string '3000' should parse)", len(result.TDFiles))
	}
}

// TestRouteTDMissingEstMinutes tests handling of missing EST_MINUTES
func TestRouteTDMissingEstMinutes(t *testing.T) {
	input := `{
		"rows": [
			{"ID": "TD-001", "CATEGORY": "test"}
		]
	}`

	cmd := newRouteTDCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--content", input, "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result RouteTDResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	// Missing EST_MINUTES defaults to backlog
	if len(result.Backlog) != 1 {
		t.Errorf("missing EST_MINUTES should route to backlog, got backlog=%d", len(result.Backlog))
	}

	// Should have a parse warning
	if len(result.ParseErrors) == 0 {
		t.Error("expected parse error for missing EST_MINUTES")
	}
}

// TestRouteTDInvalidEstMinutes tests handling of invalid EST_MINUTES
func TestRouteTDInvalidEstMinutes(t *testing.T) {
	input := `{
		"rows": [
			{"ID": "TD-001", "CATEGORY": "test", "EST_MINUTES": "unknown"}
		]
	}`

	cmd := newRouteTDCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--content", input, "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result RouteTDResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	// Invalid EST_MINUTES defaults to backlog
	if len(result.Backlog) != 1 {
		t.Errorf("invalid EST_MINUTES should route to backlog, got backlog=%d", len(result.Backlog))
	}

	// Should have a parse warning
	if len(result.ParseErrors) == 0 {
		t.Error("expected parse error for invalid EST_MINUTES")
	}
}

// TestRouteTDCustomThresholds tests --quick-wins-max and --backlog-max flags
func TestRouteTDCustomThresholds(t *testing.T) {
	input := `{
		"rows": [
			{"ID": "TD-001", "EST_MINUTES": 45},
			{"ID": "TD-002", "EST_MINUTES": 100},
			{"ID": "TD-003", "EST_MINUTES": 200}
		]
	}`

	cmd := newRouteTDCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	// Custom thresholds: quick_wins < 50, backlog < 150, td_files >= 150
	cmd.SetArgs([]string{"--content", input, "--quick-wins-max", "50", "--backlog-max", "150", "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result RouteTDResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if len(result.QuickWins) != 1 {
		t.Errorf("quick_wins = %d, want 1 (45 < 50)", len(result.QuickWins))
	}

	if len(result.Backlog) != 1 {
		t.Errorf("backlog = %d, want 1 (100 >= 50 and < 150)", len(result.Backlog))
	}

	if len(result.TDFiles) != 1 {
		t.Errorf("td_files = %d, want 1 (200 >= 150)", len(result.TDFiles))
	}
}

// TestRouteTDInvalidThresholds tests threshold validation
func TestRouteTDInvalidThresholds(t *testing.T) {
	input := `{"rows": []}`

	tests := []struct {
		name          string
		args          []string
		expectError   bool
		errorContains string
	}{
		{
			name:          "quick-wins-max > backlog-max",
			args:          []string{"--content", input, "--quick-wins-max", "5000", "--backlog-max", "2880", "--json"},
			expectError:   true,
			errorContains: "quick-wins-max",
		},
		{
			name:          "equal thresholds",
			args:          []string{"--content", input, "--quick-wins-max", "100", "--backlog-max", "100", "--json"},
			expectError:   true,
			errorContains: "must be less than",
		},
		{
			name:          "negative threshold",
			args:          []string{"--content", input, "--quick-wins-max", "-10", "--json"},
			expectError:   true,
			errorContains: "non-negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newRouteTDCmd()
			buf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetErr(buf)
			cmd.SetArgs(tt.args)

			err := cmd.Execute()

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error, got nil")
				} else if !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errorContains)
				}
			}
		})
	}
}

// TestRouteTDEmptyInput tests handling of empty input
func TestRouteTDEmptyInput(t *testing.T) {
	input := `{"rows": []}`

	cmd := newRouteTDCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--content", input, "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result RouteTDResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if result.Summary.TotalInput != 0 {
		t.Errorf("summary.total_input = %d, want 0", result.Summary.TotalInput)
	}

	if len(result.QuickWins)+len(result.Backlog)+len(result.TDFiles) != 0 {
		t.Errorf("expected all empty arrays for empty input")
	}
}

// TestRouteTDFloatEstMinutes tests float EST_MINUTES handling
func TestRouteTDFloatEstMinutes(t *testing.T) {
	input := `{
		"rows": [
			{"ID": "TD-001", "EST_MINUTES": 29.9},
			{"ID": "TD-002", "EST_MINUTES": 30.0},
			{"ID": "TD-003", "EST_MINUTES": 30.1}
		]
	}`

	cmd := newRouteTDCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--content", input, "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result RouteTDResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	// 29.9 < 30 -> quick_wins
	if len(result.QuickWins) != 1 {
		t.Errorf("quick_wins = %d, want 1 (29.9 < 30)", len(result.QuickWins))
	}

	// 30.0 and 30.1 >= 30 -> backlog
	if len(result.Backlog) != 2 {
		t.Errorf("backlog = %d, want 2 (30.0 and 30.1 >= 30)", len(result.Backlog))
	}
}

// TestRouteTDFromFile tests reading input from a file
func TestRouteTDFromFile(t *testing.T) {
	tmpDir := t.TempDir()
	inputFile := tmpDir + "/td.json"
	content := `{"rows": [{"ID": "TD-001", "EST_MINUTES": 15}]}`
	if err := os.WriteFile(inputFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create input file: %v", err)
	}

	cmd := newRouteTDCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--file", inputFile, "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result RouteTDResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if len(result.QuickWins) != 1 {
		t.Errorf("quick_wins = %d, want 1", len(result.QuickWins))
	}
}

// TestRouteTDMinimalOutput tests minimal output mode
func TestRouteTDMinimalOutput(t *testing.T) {
	input := `{"rows": [{"ID": "TD-001", "EST_MINUTES": 15}]}`

	cmd := newRouteTDCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--content", input, "--min"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Just verify it runs without error
	if buf.Len() == 0 {
		t.Error("expected some output in minimal mode")
	}
}

// TestRouteTDMissingFile tests error for non-existent file
func TestRouteTDMissingFile(t *testing.T) {
	cmd := newRouteTDCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--file", "/nonexistent/td.json", "--json"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for missing file")
	}
}

// TestRouteTDInvalidJSON tests error for invalid JSON input
func TestRouteTDInvalidJSON(t *testing.T) {
	input := `not valid json`

	cmd := newRouteTDCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--content", input, "--json"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

// TestRouteTDHumanReadableOutput tests human-readable output mode
func TestRouteTDHumanReadableOutput(t *testing.T) {
	input := `{
		"rows": [
			{"ID": "TD-001", "CATEGORY": "perf", "EST_MINUTES": 15},
			{"ID": "TD-002", "CATEGORY": "sec", "EST_MINUTES": 120},
			{"ID": "TD-003", "CATEGORY": "arch", "EST_MINUTES": 3000}
		]
	}`

	cmd := newRouteTDCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--content", input})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	// Should contain human-readable labels
	if !strings.Contains(output, "ROUTING_SUMMARY") && !strings.Contains(output, "QUICK_WINS") {
		t.Errorf("expected human-readable output, got: %s", output)
	}
}

// TestExtractEstMinutesTypes tests extractEstMinutes with different types
func TestExtractEstMinutesTypes(t *testing.T) {
	tests := []struct {
		name      string
		row       map[string]interface{}
		expected  float64
		expectErr bool
	}{
		{
			name:     "float64",
			row:      map[string]interface{}{"EST_MINUTES": float64(30.5)},
			expected: 30.5,
		},
		{
			name:     "int",
			row:      map[string]interface{}{"EST_MINUTES": int(45)},
			expected: 45.0,
		},
		{
			name:     "int64",
			row:      map[string]interface{}{"EST_MINUTES": int64(60)},
			expected: 60.0,
		},
		{
			name:     "string parseable as float",
			row:      map[string]interface{}{"EST_MINUTES": "75.5"},
			expected: 75.5,
		},
		{
			name:     "string parseable as int",
			row:      map[string]interface{}{"EST_MINUTES": "90"},
			expected: 90.0,
		},
		{
			name:      "missing field",
			row:       map[string]interface{}{"ID": "TD-001"},
			expectErr: true,
		},
		{
			name:      "invalid string",
			row:       map[string]interface{}{"EST_MINUTES": "not_a_number"},
			expectErr: true,
		},
		{
			name:      "invalid type (bool)",
			row:       map[string]interface{}{"EST_MINUTES": true},
			expectErr: true,
		},
		{
			name:     "json.Number valid",
			row:      map[string]interface{}{"EST_MINUTES": json.Number("123")},
			expected: 123.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, err := extractEstMinutes(tt.row)
			if tt.expectErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if val != tt.expected {
					t.Errorf("value = %v, want %v", val, tt.expected)
				}
			}
		})
	}
}

// TestRouteTDNoInput tests error when no input is provided
func TestRouteTDNoInput(t *testing.T) {
	cmd := newRouteTDCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--json"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when no input provided")
	}
}
