package semantic

import (
	"os/exec"
	"path/filepath"
	"strings"
)

// gitChangedFiles returns files changed (added/modified) and deleted since gitRef.
// Paths are returned relative to rootPath.
func gitChangedFiles(rootPath, gitRef string) (changed, deleted []string, err error) {
	// git diff --name-status <ref> HEAD
	cmd := exec.Command("git", "diff", "--name-status", gitRef, "HEAD")
	cmd.Dir = rootPath

	out, err := cmd.Output()
	if err != nil {
		return nil, nil, err
	}

	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			continue
		}

		status := parts[0]
		relPath := parts[1]

		// Handle renames (R100\told\tnew) — status field can be like "R100"
		if strings.HasPrefix(status, "R") {
			renameParts := strings.SplitN(relPath, "\t", 2)
			if len(renameParts) == 2 {
				deleted = append(deleted, filepath.FromSlash(renameParts[0]))
				changed = append(changed, filepath.FromSlash(renameParts[1]))
			}
			continue
		}

		path := filepath.FromSlash(relPath)
		switch status {
		case "D":
			deleted = append(deleted, path)
		case "A", "M":
			changed = append(changed, path)
		}
	}

	return changed, deleted, nil
}

// IsGitRepo checks if the given path is inside a git repository.
func IsGitRepo(path string) bool {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = path
	out, err := cmd.Output()
	return err == nil && strings.TrimSpace(string(out)) == "true"
}

// GitRepoRoot returns the root of the git repository containing path.
func GitRepoRoot(path string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = path
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
