package commands

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/samestrin/llm-tools/internal/support/gitignore"
	"github.com/samestrin/llm-tools/internal/support/utils"
	"github.com/samestrin/llm-tools/pkg/output"
	"github.com/spf13/cobra"
)

var (
	statsNoGitignore bool
	statsPath        string
	statsJSON        bool
	statsMinimal     bool
)

// newStatsCmd creates the stats command
func newStatsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Display directory statistics",
		Long: `Display statistics about a directory including file counts,
total size, and breakdown by file extension.`,
		Args: cobra.NoArgs,
		RunE: runStats,
	}
	cmd.Flags().StringVar(&statsPath, "path", ".", "Directory path to analyze")
	cmd.Flags().BoolVar(&statsNoGitignore, "no-gitignore", false, "Disable .gitignore filtering")
	cmd.Flags().BoolVar(&statsJSON, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&statsMinimal, "min", false, "Output in minimal/token-optimized format")
	return cmd
}

// ExtensionInfo represents file extension statistics.
type ExtensionInfo struct {
	Extension string `json:"extension,omitempty"`
	Ext       string `json:"ext,omitempty"`
	Count     int    `json:"count,omitempty"`
	C         int    `json:"c,omitempty"`
	Size      string `json:"size,omitempty"`
	S         string `json:"s,omitempty"`
}

// StatsResult represents the JSON output of the stats command.
type StatsResult struct {
	Path        string          `json:"path,omitempty"`
	P           string          `json:"p,omitempty"`
	Files       int             `json:"files,omitempty"`
	F           int             `json:"f,omitempty"`
	Directories int             `json:"directories,omitempty"`
	D           int             `json:"d,omitempty"`
	TotalSize   string          `json:"total_size,omitempty"`
	S           string          `json:"s,omitempty"`
	Extensions  []ExtensionInfo `json:"by_extension,omitempty"`
	Ext         []ExtensionInfo `json:"ext,omitempty"`
}

func runStats(cmd *cobra.Command, args []string) error {
	path, err := filepath.Abs(statsPath)
	if err != nil {
		return fmt.Errorf("invalid path: %s", statsPath)
	}

	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("path does not exist: %s", path)
	}

	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", path)
	}

	var ignorer *gitignore.Parser
	if !statsNoGitignore {
		ignorer, _ = gitignore.NewParser(path)
	}

	var totalFiles int
	var totalDirs int
	var totalSize int64
	extCounts := make(map[string]int)
	extSizes := make(map[string]int64)

	filepath.Walk(path, func(filePath string, fileInfo os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		// Skip hidden files unless --no-gitignore
		if !statsNoGitignore && strings.HasPrefix(fileInfo.Name(), ".") {
			if fileInfo.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Check gitignore
		if ignorer != nil && ignorer.IsIgnored(filePath) {
			if fileInfo.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if fileInfo.IsDir() {
			if filePath != path {
				totalDirs++
			}
		} else {
			totalFiles++
			size := fileInfo.Size()
			totalSize += size

			ext := strings.ToLower(filepath.Ext(fileInfo.Name()))
			if ext == "" {
				ext = "(no extension)"
			}
			extCounts[ext]++
			extSizes[ext] += size
		}

		return nil
	})

	// Build extension list sorted by count
	type extInfo struct {
		ext   string
		count int
		size  int64
	}
	var exts []extInfo
	for ext, count := range extCounts {
		exts = append(exts, extInfo{ext, count, extSizes[ext]})
	}
	sort.Slice(exts, func(i, j int) bool {
		return exts[i].count > exts[j].count
	})

	var extensions []ExtensionInfo
	for _, e := range exts {
		if statsMinimal {
			extensions = append(extensions, ExtensionInfo{Ext: e.ext, C: e.count, S: utils.FormatSize(e.size)})
		} else {
			extensions = append(extensions, ExtensionInfo{Extension: e.ext, Count: e.count, Size: utils.FormatSize(e.size)})
		}
	}

	var result StatsResult
	if statsMinimal {
		result = StatsResult{
			P:   path,
			F:   totalFiles,
			D:   totalDirs,
			S:   utils.FormatSize(totalSize),
			Ext: extensions,
		}
	} else {
		result = StatsResult{
			Path:        path,
			Files:       totalFiles,
			Directories: totalDirs,
			TotalSize:   utils.FormatSize(totalSize),
			Extensions:  extensions,
		}
	}

	formatter := output.New(statsJSON, statsMinimal, cmd.OutOrStdout())
	return formatter.Print(result, func(w io.Writer, data interface{}) {
		r := data.(StatsResult)
		p := r.Path
		if r.P != "" {
			p = r.P
		}
		f := r.Files
		if r.F > 0 {
			f = r.F
		}
		d := r.Directories
		if r.D > 0 {
			d = r.D
		}
		s := r.TotalSize
		if r.S != "" {
			s = r.S
		}
		ext := r.Extensions
		if r.Ext != nil {
			ext = r.Ext
		}

		fmt.Fprintf(w, "PATH: %s\n", p)
		fmt.Fprintf(w, "FILES: %d\n", f)
		fmt.Fprintf(w, "DIRECTORIES: %d\n", d)
		fmt.Fprintf(w, "TOTAL_SIZE: %s\n", s)
		fmt.Fprintln(w)

		if len(ext) > 0 {
			fmt.Fprintln(w, "BY_EXTENSION:")
			for _, e := range ext {
				extName := e.Extension
				if e.Ext != "" {
					extName = e.Ext
				}
				count := e.Count
				if e.C > 0 {
					count = e.C
				}
				size := e.Size
				if e.S != "" {
					size = e.S
				}
				fmt.Fprintf(w, "  %s: %d files (%s)\n", extName, count, size)
			}
		}
	})
}

func init() {
	RootCmd.AddCommand(newStatsCmd())
}
