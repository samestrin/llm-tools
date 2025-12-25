package gitignore

import (
	"path/filepath"
	"strings"

	ignore "github.com/sabhiram/go-gitignore"
)

// Parser handles .gitignore pattern matching
type Parser struct {
	rootPath string
	ignorer  *ignore.GitIgnore
}

// NewParser creates a new gitignore parser for the given root path
func NewParser(rootPath string) (*Parser, error) {
	absPath, err := filepath.Abs(rootPath)
	if err != nil {
		return nil, err
	}

	// Load .gitignore from the root path
	gitignorePath := filepath.Join(absPath, ".gitignore")
	ignorer, err := ignore.CompileIgnoreFile(gitignorePath)
	if err != nil {
		// If no .gitignore exists, create an empty matcher
		ignorer = ignore.CompileIgnoreLines()
	}

	return &Parser{
		rootPath: absPath,
		ignorer:  ignorer,
	}, nil
}

// IsIgnored checks if the given path should be ignored
func (p *Parser) IsIgnored(path string) bool {
	if p.ignorer == nil {
		return false
	}

	// Get absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	// Check if path is within root
	if !strings.HasPrefix(absPath, p.rootPath) {
		return false
	}

	// Get relative path from root
	relPath, err := filepath.Rel(p.rootPath, absPath)
	if err != nil {
		return false
	}

	// Handle paths that resolve to current directory or parent
	if relPath == "." || strings.HasPrefix(relPath, "..") {
		return false
	}

	// Check if the path matches
	if p.ignorer.MatchesPath(relPath) {
		return true
	}

	// Also try with trailing slash for directory patterns
	if p.ignorer.MatchesPath(relPath + "/") {
		return true
	}

	return false
}

// RootPath returns the root path of the parser
func (p *Parser) RootPath() string {
	return p.rootPath
}
