package pathvalidation

import (
	"errors"
	"testing"
)

func TestCheckUnresolvedTemplateVars(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		wantErr     bool
		wantPattern string
	}{
		// Valid paths (no template variables)
		{
			name:    "simple path",
			path:    "/home/user/projects",
			wantErr: false,
		},
		{
			name:    "path with numbers",
			path:    "115.0-feature-name",
			wantErr: false,
		},
		{
			name:    "path with dashes and underscores",
			path:    "my-project_v2",
			wantErr: false,
		},
		{
			name:    "empty string",
			path:    "",
			wantErr: false,
		},

		// Double brace templates {{VAR}}
		{
			name:        "double brace template",
			path:        "{{NEXT}}-feature",
			wantErr:     true,
			wantPattern: "double-brace",
		},
		{
			name:        "double brace with spaces",
			path:        "{{ NEXT }}-feature",
			wantErr:     true,
			wantPattern: "double-brace",
		},
		{
			name:        "double brace in middle",
			path:        "/plans/{{VERSION}}/user-stories",
			wantErr:     true,
			wantPattern: "double-brace",
		},

		// GitHub Actions style ${{VAR}}
		{
			name:        "github actions style",
			path:        "${{NEXT}}-feature",
			wantErr:     true,
			wantPattern: "github-actions",
		},
		{
			name:        "github actions with spaces",
			path:        "${{ env.VERSION }}-release",
			wantErr:     true,
			wantPattern: "github-actions",
		},

		// Shell brace expansion ${VAR}
		{
			name:        "shell brace var",
			path:        "${NEXT}-feature",
			wantErr:     true,
			wantPattern: "shell-brace",
		},
		{
			name:        "shell brace with default",
			path:        "${NEXT:-1.0}-feature",
			wantErr:     true,
			wantPattern: "shell-brace",
		},

		// Shell variable $VAR
		{
			name:        "shell var uppercase",
			path:        "$NEXT-feature",
			wantErr:     true,
			wantPattern: "shell-var",
		},
		{
			name:        "shell var with underscore",
			path:        "$PLAN_VERSION-feature",
			wantErr:     true,
			wantPattern: "shell-var",
		},
		{
			name:        "shell var HOME",
			path:        "$HOME/projects",
			wantErr:     true,
			wantPattern: "shell-var",
		},

		// Double bracket variables [[VAR]]
		{
			name:        "double bracket var",
			path:        "[[NEXT]]-feature",
			wantErr:     true,
			wantPattern: "double-bracket-var",
		},
		{
			name:        "double bracket var with content",
			path:        "[[plan.version]]-release",
			wantErr:     true,
			wantPattern: "double-bracket-var",
		},

		// Single bracket variables [VAR] - uppercase only
		{
			name:        "single bracket uppercase var",
			path:        "[NEXT]-feature",
			wantErr:     true,
			wantPattern: "single-bracket-var",
		},
		{
			name:        "single bracket with underscore",
			path:        "[PLAN_VERSION]-release",
			wantErr:     true,
			wantPattern: "single-bracket-var",
		},

		// Edge cases - should NOT trigger
		{
			name:    "single dollar sign",
			path:    "price-$5",
			wantErr: false,
		},
		{
			name:    "lowercase after dollar",
			path:    "$lowercase",
			wantErr: false,
		},
		{
			name:    "single brace",
			path:    "{not-a-template}",
			wantErr: false,
		},
		{
			name:    "single bracket lowercase",
			path:    "[lowercase]",
			wantErr: false,
		},
		{
			name:    "single bracket mixed case",
			path:    "[MixedCase]",
			wantErr: false,
		},
		{
			name:    "single bracket number",
			path:    "[0]",
			wantErr: false,
		},
		{
			name:    "array index style",
			path:    "items[0]",
			wantErr: false,
		},
		{
			name:    "optional marker",
			path:    "[optional]-param",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckUnresolvedTemplateVars(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckUnresolvedTemplateVars() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.wantPattern != "" {
				var templateErr *UnresolvedTemplateError
				if errors.As(err, &templateErr) {
					if templateErr.Pattern != tt.wantPattern {
						t.Errorf("CheckUnresolvedTemplateVars() pattern = %v, want %v", templateErr.Pattern, tt.wantPattern)
					}
				} else {
					t.Errorf("CheckUnresolvedTemplateVars() expected UnresolvedTemplateError, got %T", err)
				}
			}
		})
	}
}

func TestCheckPathComponents(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name:    "valid full path",
			path:    "/home/user/projects/115.0-feature",
			wantErr: false,
		},
		{
			name:    "template in last component",
			path:    "/home/user/projects/{{NEXT}}-feature",
			wantErr: true,
		},
		{
			name:    "template in middle component",
			path:    "/home/user/{{PROJECT}}/feature",
			wantErr: true,
		},
		{
			name:    "template in first component",
			path:    "{{ROOT}}/user/projects",
			wantErr: true,
		},
		{
			name:    "windows style path",
			path:    "C:\\Users\\{{USER}}\\projects",
			wantErr: true,
		},
		{
			name:    "mixed separators",
			path:    "/home/user\\{{PROJECT}}/feature",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckPathComponents(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckPathComponents() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidatePathForCreation(t *testing.T) {
	// ValidatePathForCreation is a wrapper, so just test basic functionality
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"valid", "/tmp/test/115.0-feature", false},
		{"invalid template", "/tmp/test/{{NEXT}}-feature", true},
		{"invalid shell var", "/tmp/$PLAN/feature", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePathForCreation(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePathForCreation() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestUnresolvedTemplateError_Error(t *testing.T) {
	err := &UnresolvedTemplateError{
		Path:     "/test/{{NEXT}}/feature",
		Variable: "{{NEXT}}",
		Pattern:  "double-brace",
	}

	expected := "path contains unresolved template variable '{{NEXT}}' - check your variable substitution"
	if err.Error() != expected {
		t.Errorf("Error() = %v, want %v", err.Error(), expected)
	}
}
