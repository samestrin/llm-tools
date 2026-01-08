// Package pathvalidation provides validation utilities for file paths
package pathvalidation

import (
	"fmt"
	"regexp"
	"strings"
)

// UnresolvedTemplateError indicates a path contains unresolved template variables
type UnresolvedTemplateError struct {
	Path     string
	Variable string
	Pattern  string
}

func (e *UnresolvedTemplateError) Error() string {
	return fmt.Sprintf("path contains unresolved template variable '%s' - check your variable substitution", e.Variable)
}

// templatePatterns defines patterns that indicate unresolved template variables.
// Order matters - more specific patterns must come before more general ones.
var templatePatterns = []struct {
	name    string
	pattern *regexp.Regexp
}{
	{"github-actions", regexp.MustCompile(`\$\{\{[^}]*\}\}`)},          // ${{VAR}} or ${{ VAR }} - must come before double-brace
	{"double-brace", regexp.MustCompile(`\{\{[^}]*\}\}`)},              // {{VAR}} or {{ VAR }}
	{"shell-brace", regexp.MustCompile(`\$\{[^}]+\}`)},                 // ${VAR}
	{"shell-var", regexp.MustCompile(`\$[A-Z_][A-Z0-9_]*`)},            // $VAR or $VAR_NAME
	{"double-bracket-var", regexp.MustCompile(`\[\[[^\]]+\]\]`)},       // [[VAR]]
	{"single-bracket-var", regexp.MustCompile(`\[[A-Z_][A-Z0-9_]*\]`)}, // [VAR] or [VAR_NAME] - uppercase only
}

// CheckUnresolvedTemplateVars checks if a path contains unresolved template variables.
// Returns an error if any template-like patterns are found.
func CheckUnresolvedTemplateVars(path string) error {
	for _, tp := range templatePatterns {
		if match := tp.pattern.FindString(path); match != "" {
			return &UnresolvedTemplateError{
				Path:     path,
				Variable: match,
				Pattern:  tp.name,
			}
		}
	}
	return nil
}

// CheckPathComponents checks each component of a path for unresolved template variables.
// This is useful when you want to identify which specific directory component has the issue.
func CheckPathComponents(path string) error {
	// Normalize separators
	normalized := strings.ReplaceAll(path, "\\", "/")
	components := strings.Split(normalized, "/")

	for _, component := range components {
		if component == "" {
			continue
		}
		if err := CheckUnresolvedTemplateVars(component); err != nil {
			return err
		}
	}
	return nil
}

// ValidatePathForCreation performs all validation checks suitable for path creation operations.
// This includes checking for unresolved template variables.
func ValidatePathForCreation(path string) error {
	return CheckPathComponents(path)
}
