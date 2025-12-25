package commands

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

var hashAlgorithm string

// newHashCmd creates the hash command
func newHashCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hash [paths...]",
		Short: "Generate hash checksums for files",
		Long: `Generate hash checksums for one or more files.

Supported algorithms:
  md5    - MD5 hash
  sha1   - SHA-1 hash
  sha256 - SHA-256 hash (default)
  sha512 - SHA-512 hash`,
		Args: cobra.MinimumNArgs(1),
		RunE: runHash,
	}
	cmd.Flags().StringVarP(&hashAlgorithm, "algorithm", "a", "sha256", "Hash algorithm: md5, sha1, sha256, sha512")
	return cmd
}

func runHash(cmd *cobra.Command, args []string) error {
	algo := strings.ToLower(hashAlgorithm)

	// Validate algorithm
	if algo != "md5" && algo != "sha1" && algo != "sha256" && algo != "sha512" {
		return fmt.Errorf("unsupported algorithm: %s (supported: md5, sha1, sha256, sha512)", algo)
	}

	// Collect files
	var files []string
	for _, pathArg := range args {
		absPath, err := filepath.Abs(pathArg)
		if err != nil {
			return fmt.Errorf("invalid path: %s", pathArg)
		}

		info, err := os.Stat(absPath)
		if err != nil {
			return fmt.Errorf("path not found: %s", pathArg)
		}

		if info.IsDir() {
			// Recursively collect files from directory
			err = filepath.Walk(absPath, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return nil // Skip on error
				}
				if !info.IsDir() {
					files = append(files, path)
				}
				return nil
			})
			if err != nil {
				return err
			}
		} else {
			files = append(files, absPath)
		}
	}

	if len(files) == 0 {
		return fmt.Errorf("no files found")
	}

	// Deduplicate and sort
	seen := make(map[string]bool)
	uniqueFiles := []string{}
	for _, f := range files {
		if !seen[f] {
			seen[f] = true
			uniqueFiles = append(uniqueFiles, f)
		}
	}
	sort.Strings(uniqueFiles)

	// Calculate hashes
	for _, filePath := range uniqueFiles {
		hashValue, err := computeHash(filePath, algo)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "ERROR: %s: %v\n", filePath, err)
			continue
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s  %s\n", hashValue, filePath)
	}

	return nil
}

func computeHash(filePath string, algo string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	var hasher hash.Hash
	switch algo {
	case "md5":
		hasher = md5.New()
	case "sha1":
		hasher = sha1.New()
	case "sha256":
		hasher = sha256.New()
	case "sha512":
		hasher = sha512.New()
	default:
		return "", fmt.Errorf("unsupported algorithm: %s", algo)
	}

	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hasher.Sum(nil)), nil
}

func init() {
	RootCmd.AddCommand(newHashCmd())
}
