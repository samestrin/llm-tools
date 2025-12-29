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

	"github.com/samestrin/llm-tools/pkg/output"
	"github.com/spf13/cobra"
)

var (
	hashAlgorithm string
	hashJSON      bool
	hashMinimal   bool
)

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
	cmd.Flags().BoolVar(&hashJSON, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&hashMinimal, "min", false, "Output in minimal/token-optimized format")
	return cmd
}

// HashEntry represents a single file hash result.
type HashEntry struct {
	File string `json:"file,omitempty"`
	F    string `json:"f,omitempty"`
	Hash string `json:"hash,omitempty"`
	H    string `json:"h,omitempty"`
}

// HashResult represents the JSON output of the hash command.
type HashResult struct {
	Algorithm string      `json:"algorithm,omitempty"`
	Algo      string      `json:"algo,omitempty"`
	Hashes    []HashEntry `json:"hashes,omitempty"`
	H         []HashEntry `json:"h,omitempty"`
	Count     int         `json:"count,omitempty"`
	C         int         `json:"c,omitempty"`
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
	var entries []HashEntry
	for _, filePath := range uniqueFiles {
		hashValue, err := computeHash(filePath, algo)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "ERROR: %s: %v\n", filePath, err)
			continue
		}
		if hashMinimal {
			entries = append(entries, HashEntry{F: filePath, H: hashValue})
		} else {
			entries = append(entries, HashEntry{File: filePath, Hash: hashValue})
		}
	}

	var result HashResult
	if hashMinimal {
		result = HashResult{Algo: algo, H: entries, C: len(entries)}
	} else {
		result = HashResult{Algorithm: algo, Hashes: entries, Count: len(entries)}
	}

	formatter := output.New(hashJSON, hashMinimal, cmd.OutOrStdout())
	return formatter.Print(result, func(w io.Writer, data interface{}) {
		r := data.(HashResult)
		hashes := r.Hashes
		if r.H != nil {
			hashes = r.H
		}
		for _, e := range hashes {
			file := e.File
			if e.F != "" {
				file = e.F
			}
			h := e.Hash
			if e.H != "" {
				h = e.H
			}
			fmt.Fprintf(w, "%s  %s\n", h, file)
		}
	})
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
