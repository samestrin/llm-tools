package semantic

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"
)

// IndexLock provides cross-process file locking for index operations.
// Prevents concurrent index/index-update processes from corrupting shared state.
type IndexLock struct {
	flock *flock.Flock
	path  string
}

// NewIndexLock creates a lock for the given index directory and collection.
// For SQLite: lockPath is derived from the index file path (sibling .lock file).
// For Qdrant: lockPath is derived from data dir + collection name.
func NewIndexLock(indexPath, storageType, collectionName string) *IndexLock {
	var lockPath string
	if storageType == "qdrant" {
		// For Qdrant, use a per-collection lock in a temp-safe location
		lockDir := os.TempDir()
		lockPath = filepath.Join(lockDir, fmt.Sprintf(".semantic-%s.lock", collectionName))
	} else {
		// For SQLite, lock file sits next to the index DB
		lockPath = indexPath + ".lock"
	}

	return &IndexLock{
		flock: flock.New(lockPath),
		path:  lockPath,
	}
}

// Lock acquires an exclusive lock, blocking up to timeout.
// Use for full index rebuilds that must wait for other operations to finish.
func (l *IndexLock) Lock(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	locked, err := l.flock.TryLockContext(ctx, 500*time.Millisecond)
	if err != nil {
		return fmt.Errorf("failed to acquire index lock %s: %w", l.path, err)
	}
	if !locked {
		return fmt.Errorf("timed out waiting for index lock %s after %v", l.path, timeout)
	}
	return nil
}

// TryLock attempts to acquire an exclusive lock without blocking.
// Returns true if lock was acquired, false if another process holds it.
// Use for index-update operations that should silently skip if busy.
func (l *IndexLock) TryLock() (bool, error) {
	return l.flock.TryLock()
}

// Unlock releases the lock.
func (l *IndexLock) Unlock() error {
	return l.flock.Unlock()
}

// Path returns the lock file path (useful for diagnostics).
func (l *IndexLock) Path() string {
	return l.path
}
