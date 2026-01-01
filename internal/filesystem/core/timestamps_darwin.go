//go:build darwin

package core

import (
	"os"
	"syscall"
	"time"
)

// GetFileTimestamps returns the created, accessed, and modified timestamps for a file
// On Darwin (macOS), we can get all three timestamps from the stat structure
func GetFileTimestamps(info os.FileInfo) (created, accessed, modified time.Time) {
	modified = info.ModTime()

	sys := info.Sys()
	if sys == nil {
		return modified, modified, modified
	}

	stat, ok := sys.(*syscall.Stat_t)
	if !ok {
		return modified, modified, modified
	}

	// Darwin provides Birthtimespec for creation time
	created = time.Unix(stat.Birthtimespec.Sec, stat.Birthtimespec.Nsec)

	// Access time
	accessed = time.Unix(stat.Atimespec.Sec, stat.Atimespec.Nsec)

	return created, accessed, modified
}
