package commands

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func resetRuntimeFlags() {
	runtimeStart = 0
	runtimeEnd = 0
	runtimeFormat = "human"
	runtimePrecision = 1
	runtimeLabel = false
	runtimeRaw = false
	runtimeJSON = false
	runtimeMinimal = false
}

func TestRuntimeCommand(t *testing.T) {
	now := time.Now().Unix()

	t.Run("basic runtime calculation", func(t *testing.T) {
		resetRuntimeFlags()

		cmd := newRuntimeCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetArgs([]string{"--start", "1735800000", "--end", "1735800127"})

		err := cmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		output := strings.TrimSpace(buf.String())
		// 127 seconds = 2m 7.0s
		if !strings.Contains(output, "2m") || !strings.Contains(output, "7") {
			t.Errorf("expected output to contain '2m' and '7', got: %s", output)
		}
	})

	t.Run("format secs", func(t *testing.T) {
		resetRuntimeFlags()

		cmd := newRuntimeCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetArgs([]string{"--start", "1735800000", "--end", "1735800127", "--format", "secs"})

		err := cmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		output := strings.TrimSpace(buf.String())
		if output != "127.0s" {
			t.Errorf("expected '127.0s', got: %s", output)
		}
	})

	t.Run("format mins", func(t *testing.T) {
		resetRuntimeFlags()

		cmd := newRuntimeCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetArgs([]string{"--start", "1735800000", "--end", "1735800120", "--format", "mins"})

		err := cmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		output := strings.TrimSpace(buf.String())
		if output != "2.0m" {
			t.Errorf("expected '2.0m', got: %s", output)
		}
	})

	t.Run("format mins-secs", func(t *testing.T) {
		resetRuntimeFlags()

		cmd := newRuntimeCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetArgs([]string{"--start", "1735800000", "--end", "1735800127", "--format", "mins-secs"})

		err := cmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		output := strings.TrimSpace(buf.String())
		if output != "2m 7.0s" {
			t.Errorf("expected '2m 7.0s', got: %s", output)
		}
	})

	t.Run("format hms", func(t *testing.T) {
		resetRuntimeFlags()

		cmd := newRuntimeCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		// 1h 23m 45s = 5025 seconds
		cmd.SetArgs([]string{"--start", "1735800000", "--end", "1735805025", "--format", "hms"})

		err := cmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		output := strings.TrimSpace(buf.String())
		if output != "1h 23m 45.0s" {
			t.Errorf("expected '1h 23m 45.0s', got: %s", output)
		}
	})

	t.Run("format compact", func(t *testing.T) {
		resetRuntimeFlags()

		cmd := newRuntimeCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetArgs([]string{"--start", "1735800000", "--end", "1735800127", "--format", "compact"})

		err := cmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		output := strings.TrimSpace(buf.String())
		if output != "2m7.0s" {
			t.Errorf("expected '2m7.0s', got: %s", output)
		}
	})

	t.Run("format human short duration", func(t *testing.T) {
		resetRuntimeFlags()

		cmd := newRuntimeCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetArgs([]string{"--start", "1735800000", "--end", "1735800030", "--format", "human"})

		err := cmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		output := strings.TrimSpace(buf.String())
		if output != "30.0s" {
			t.Errorf("expected '30.0s', got: %s", output)
		}
	})

	t.Run("format human hours", func(t *testing.T) {
		resetRuntimeFlags()

		cmd := newRuntimeCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		// 2 hours = 7200 seconds
		cmd.SetArgs([]string{"--start", "1735800000", "--end", "1735807200", "--format", "human"})

		err := cmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		output := strings.TrimSpace(buf.String())
		if output != "2h" {
			t.Errorf("expected '2h', got: %s", output)
		}
	})

	t.Run("format human days", func(t *testing.T) {
		resetRuntimeFlags()

		cmd := newRuntimeCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		// 2 days 3 hours = 183600 seconds
		cmd.SetArgs([]string{"--start", "1735800000", "--end", "1735983600", "--format", "human"})

		err := cmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		output := strings.TrimSpace(buf.String())
		if output != "2d 3h" {
			t.Errorf("expected '2d 3h', got: %s", output)
		}
	})

	t.Run("precision 0", func(t *testing.T) {
		resetRuntimeFlags()

		cmd := newRuntimeCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetArgs([]string{"--start", "1735800000", "--end", "1735800127", "--format", "secs", "--precision", "0"})

		err := cmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		output := strings.TrimSpace(buf.String())
		if output != "127s" {
			t.Errorf("expected '127s', got: %s", output)
		}
	})

	t.Run("precision 2", func(t *testing.T) {
		resetRuntimeFlags()

		cmd := newRuntimeCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetArgs([]string{"--start", "1735800000", "--end", "1735800127", "--format", "secs", "--precision", "2"})

		err := cmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		output := strings.TrimSpace(buf.String())
		if output != "127.00s" {
			t.Errorf("expected '127.00s', got: %s", output)
		}
	})

	t.Run("label option", func(t *testing.T) {
		resetRuntimeFlags()

		cmd := newRuntimeCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetArgs([]string{"--start", "1735800000", "--end", "1735800030", "--label"})

		err := cmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		output := strings.TrimSpace(buf.String())
		if !strings.HasPrefix(output, "Runtime: ") {
			t.Errorf("expected output to start with 'Runtime: ', got: %s", output)
		}
	})

	t.Run("raw option secs", func(t *testing.T) {
		resetRuntimeFlags()

		cmd := newRuntimeCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetArgs([]string{"--start", "1735800000", "--end", "1735800127", "--format", "secs", "--raw"})

		err := cmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		output := strings.TrimSpace(buf.String())
		if output != "127.0" {
			t.Errorf("expected '127.0', got: %s", output)
		}
	})

	t.Run("raw option mins", func(t *testing.T) {
		resetRuntimeFlags()

		cmd := newRuntimeCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetArgs([]string{"--start", "1735800000", "--end", "1735800120", "--format", "mins", "--raw"})

		err := cmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		output := strings.TrimSpace(buf.String())
		if output != "2.0" {
			t.Errorf("expected '2.0', got: %s", output)
		}
	})

	t.Run("json output", func(t *testing.T) {
		resetRuntimeFlags()

		cmd := newRuntimeCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetArgs([]string{"--start", "1735800000", "--end", "1735800127", "--json"})

		err := cmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var result RuntimeResult
		if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}

		if result.Start != 1735800000 {
			t.Errorf("expected start 1735800000, got: %d", result.Start)
		}
		if result.End != 1735800127 {
			t.Errorf("expected end 1735800127, got: %d", result.End)
		}
		if result.ElapsedSecs != 127 {
			t.Errorf("expected elapsed_secs 127, got: %f", result.ElapsedSecs)
		}
	})

	t.Run("minimal output", func(t *testing.T) {
		resetRuntimeFlags()

		cmd := newRuntimeCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetArgs([]string{"--start", "1735800000", "--end", "1735800127", "--json", "--min"})

		err := cmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var result RuntimeResult
		if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}

		if result.S != 1735800000 {
			t.Errorf("expected s 1735800000, got: %d", result.S)
		}
		if result.E != 1735800127 {
			t.Errorf("expected e 1735800127, got: %d", result.E)
		}
		if result.ES != 127 {
			t.Errorf("expected es 127, got: %f", result.ES)
		}
	})

	t.Run("default end to now", func(t *testing.T) {
		resetRuntimeFlags()

		start := now - 60 // 60 seconds ago

		cmd := newRuntimeCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetArgs([]string{"--start", string(rune(start)), "--json"})

		// Use a fresh command with actual numeric start
		cmd2 := newRuntimeCmd()
		buf2 := new(bytes.Buffer)
		cmd2.SetOut(buf2)

		// Set start directly via flag parsing workaround
		runtimeStart = start
		runtimeEnd = 0
		runtimeJSON = true
		err := runRuntime(cmd2, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var result RuntimeResult
		if err := json.Unmarshal(buf2.Bytes(), &result); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}

		// ElapsedSecs should be approximately 60 (might be off by a second due to execution time)
		if result.ElapsedSecs < 59 || result.ElapsedSecs > 62 {
			t.Errorf("expected elapsed_secs around 60, got: %f", result.ElapsedSecs)
		}
	})

	t.Run("invalid start timestamp", func(t *testing.T) {
		resetRuntimeFlags()
		runtimeStart = -1
		runtimeEnd = 1735800127

		cmd := newRuntimeCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)

		err := runRuntime(cmd, nil)
		if err == nil {
			t.Error("expected error for invalid start timestamp")
		}
	})

	t.Run("negative duration", func(t *testing.T) {
		resetRuntimeFlags()

		cmd := newRuntimeCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		// End before start
		cmd.SetArgs([]string{"--start", "1735800127", "--end", "1735800000", "--format", "secs"})

		err := cmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		output := strings.TrimSpace(buf.String())
		if output != "-127.0s" {
			t.Errorf("expected '-127.0s', got: %s", output)
		}
	})

	t.Run("zero duration", func(t *testing.T) {
		resetRuntimeFlags()

		cmd := newRuntimeCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetArgs([]string{"--start", "1735800000", "--end", "1735800000", "--format", "secs"})

		err := cmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		output := strings.TrimSpace(buf.String())
		if output != "0.0s" {
			t.Errorf("expected '0.0s', got: %s", output)
		}
	})
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name      string
		secs      float64
		format    string
		precision int
		raw       bool
		expected  string
	}{
		// Secs format
		{"secs basic", 127.5, "secs", 1, false, "127.5s"},
		{"secs raw", 127.5, "secs", 1, true, "127.5"},
		{"secs precision 0", 127.5, "secs", 0, false, "128s"},

		// Mins format
		{"mins basic", 120, "mins", 1, false, "2.0m"},
		{"mins raw", 120, "mins", 1, true, "2.0"},
		{"mins fractional", 90, "mins", 2, false, "1.50m"},

		// Mins-secs format
		{"mins-secs basic", 127, "mins-secs", 1, false, "2m 7.0s"},
		{"mins-secs only secs", 30, "mins-secs", 1, false, "30.0s"},
		{"mins-secs raw", 127, "mins-secs", 1, true, "127.0"},

		// HMS format
		{"hms basic", 5025, "hms", 1, false, "1h 23m 45.0s"},
		{"hms only mins", 127, "hms", 1, false, "2m 7.0s"},
		{"hms only secs", 30, "hms", 1, false, "30.0s"},

		// Compact format
		{"compact basic", 5025, "compact", 1, false, "1h23m45.0s"},
		{"compact mins only", 127, "compact", 1, false, "2m7.0s"},
		{"compact secs only", 30, "compact", 1, false, "30.0s"},

		// Human format
		{"human secs", 30, "human", 1, false, "30.0s"},
		{"human mins", 127, "human", 1, false, "2m 7.0s"},
		{"human hours", 7200, "human", 1, false, "2h"},
		{"human hours mins", 7320, "human", 1, false, "2h 2m"},
		{"human days", 172800, "human", 1, false, "2d"},
		{"human days hours", 183600, "human", 1, false, "2d 3h"},

		// Negative durations
		{"negative secs", -30, "secs", 1, false, "-30.0s"},
		{"negative mins-secs", -127, "mins-secs", 1, false, "-2m 7.0s"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatDuration(tt.secs, tt.format, tt.precision, tt.raw)
			if result != tt.expected {
				t.Errorf("formatDuration(%v, %q, %d, %v) = %q, want %q",
					tt.secs, tt.format, tt.precision, tt.raw, result, tt.expected)
			}
		})
	}
}

func TestFormatFloat(t *testing.T) {
	tests := []struct {
		val       float64
		precision int
		expected  string
	}{
		{127.0, 1, "127.0"},
		{127.5, 1, "127.5"},
		{127.56, 1, "127.6"},
		{127.0, 0, "127"},
		{127.5, 0, "128"},
		{127.0, 2, "127.00"},
		{127.567, 2, "127.57"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatFloat(tt.val, tt.precision)
			if result != tt.expected {
				t.Errorf("formatFloat(%v, %d) = %q, want %q",
					tt.val, tt.precision, result, tt.expected)
			}
		})
	}
}

func TestGetRawValue(t *testing.T) {
	tests := []struct {
		name     string
		secs     float64
		format   string
		expected float64
	}{
		{"secs", 127, "secs", 127},
		{"mins", 120, "mins", 2},
		{"human secs", 30, "human", 30},
		{"human mins", 120, "human", 2},
		{"human hours", 7200, "human", 2},
		{"human days", 172800, "human", 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getRawValue(tt.secs, tt.format)
			if result != tt.expected {
				t.Errorf("getRawValue(%v, %q) = %v, want %v",
					tt.secs, tt.format, result, tt.expected)
			}
		})
	}
}
