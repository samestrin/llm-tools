//go:build linux

package core

import (
	"os"
	"syscall"
	"time"
)

// GetFileTimestamps returns the created, accessed, and modified timestamps for a file
// On Linux, creation time (birth time) is not always available, so we use ctime as fallback
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

	// Linux doesn't reliably provide birth time, use ctime (status change time) as fallback
	// Some filesystems on newer kernels support statx() which has birth time, but
	// we'll use ctime for compatibility
	created = time.Unix(stat.Ctim.Sec, stat.Ctim.Nsec)

	// Access time
	accessed = time.Unix(stat.Atim.Sec, stat.Atim.Nsec)

	return created, accessed, modified
}
