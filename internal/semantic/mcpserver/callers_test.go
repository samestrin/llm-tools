package mcpserver

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildCallersArgs(t *testing.T) {
	tests := []struct {
		name string
		args map[string]interface{}
		want []string
	}{
		{
			name: "symbol only",
			args: map[string]interface{}{"symbol": "ResolveRefs"},
			want: []string{"callers", "ResolveRefs"},
		},
		{
			name: "symbol with backend selection",
			args: map[string]interface{}{
				"symbol":     "ResolveRefs",
				"storage":    "qdrant",
				"collection": "code",
			},
			want: []string{"callers", "ResolveRefs", "--storage", "qdrant", "--collection", "code"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildCallersArgs(tt.args)
			if strings.Join(got, " ") != strings.Join(tt.want, " ") {
				t.Errorf("buildCallersArgs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateCallersArgs(t *testing.T) {
	tests := []struct {
		name      string
		args      map[string]interface{}
		wantError bool
	}{
		{name: "valid symbol", args: map[string]interface{}{"symbol": "ResolveRefs"}, wantError: false},
		{name: "missing symbol", args: map[string]interface{}{}, wantError: true},
		{name: "empty symbol", args: map[string]interface{}{"symbol": ""}, wantError: true},
		{name: "whitespace symbol", args: map[string]interface{}{"symbol": "   "}, wantError: true},
		{name: "wrong type", args: map[string]interface{}{"symbol": 42}, wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCallersArgs(tt.args)
			if (err != nil) != tt.wantError {
				t.Errorf("validateCallersArgs() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestCallersCommandRegistered(t *testing.T) {
	found := false
	for _, cmd := range RegisteredCommands() {
		if cmd == "callers" {
			found = true
		}
	}
	if !found {
		t.Error("callers is not in the command registry, so the tool cannot be dispatched")
	}
}

func TestCallersToolDefinition(t *testing.T) {
	var tool *ToolDefinition
	for i, td := range GetToolDefinitions() {
		if td.Name == ToolPrefix+"callers" {
			tool = &GetToolDefinitions()[i]
			break
		}
	}
	if tool == nil {
		t.Fatalf("%scallers is not exposed as a tool", ToolPrefix)
	}

	if tool.Description == "" {
		t.Error("callers tool needs a description")
	}

	var schema struct {
		Type       string                            `json:"type"`
		Properties map[string]map[string]interface{} `json:"properties"`
		Required   []string                          `json:"required"`
	}
	if err := json.Unmarshal(tool.InputSchema, &schema); err != nil {
		t.Fatalf("callers schema is not valid JSON: %v", err)
	}

	if _, ok := schema.Properties["symbol"]; !ok {
		t.Errorf("callers schema is missing the symbol property: %v", schema.Properties)
	}

	requiresSymbol := false
	for _, r := range schema.Required {
		if r == "symbol" {
			requiresSymbol = true
		}
	}
	if !requiresSymbol {
		t.Errorf("callers schema should require symbol, required = %v", schema.Required)
	}
}
