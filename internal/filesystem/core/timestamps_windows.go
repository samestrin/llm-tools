//go:build windows

package core

import (
	"os"
	"syscall"
	"time"
)

// GetFileTimestamps returns the created, accessed, and modified timestamps for a file
// On Windows, we can get all three timestamps from the file attributes
func GetFileTimestamps(info os.FileInfo) (created, accessed, modified time.Time) {
	modified = info.ModTime()

	sys := info.Sys()
	if sys == nil {
		return modified, modified, modified
	}

	fileAttr, ok := sys.(*syscall.Win32FileAttributeData)
	if !ok {
		return modified, modified, modified
	}

	// Windows provides creation time
	created = time.Unix(0, fileAttr.CreationTime.Nanoseconds())

	// Access time
	accessed = time.Unix(0, fileAttr.LastAccessTime.Nanoseconds())

	return created, accessed, modified
}
