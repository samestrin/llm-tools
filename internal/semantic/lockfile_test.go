package semantic

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewIndexLock_SQLite(t *testing.T) {
	tmpDir := t.TempDir()
	indexPath := filepath.Join(tmpDir, "semantic.db")

	lock := NewIndexLock(indexPath, "sqlite", "")
	expected := indexPath + ".lock"
	if lock.Path() != expected {
		t.Errorf("expected lock path %q, got %q", expected, lock.Path())
	}
}

func TestNewIndexLock_Qdrant(t *testing.T) {
	lock := NewIndexLock("", "qdrant", "my_collection")
	expected := filepath.Join(os.TempDir(), ".semantic-my_collection.lock")
	if lock.Path() != expected {
		t.Errorf("expected lock path %q, got %q", expected, lock.Path())
	}
}

func TestIndexLock_LockUnlock(t *testing.T) {
	tmpDir := t.TempDir()
	indexPath := filepath.Join(tmpDir, "semantic.db")

	lock := NewIndexLock(indexPath, "sqlite", "")
	if err := lock.Lock(5 * time.Second); err != nil {
		t.Fatalf("failed to acquire lock: %v", err)
	}
	if err := lock.Unlock(); err != nil {
		t.Fatalf("failed to release lock: %v", err)
	}
}

func TestIndexLock_TryLock(t *testing.T) {
	tmpDir := t.TempDir()
	indexPath := filepath.Join(tmpDir, "semantic.db")

	lock := NewIndexLock(indexPath, "sqlite", "")
	locked, err := lock.TryLock()
	if err != nil {
		t.Fatalf("TryLock failed: %v", err)
	}
	if !locked {
		t.Fatal("expected TryLock to succeed on fresh lock")
	}
	defer lock.Unlock()
}

func TestIndexLock_TryLock_Contention(t *testing.T) {
	tmpDir := t.TempDir()
	indexPath := filepath.Join(tmpDir, "semantic.db")

	// First lock holder
	lock1 := NewIndexLock(indexPath, "sqlite", "")
	locked, err := lock1.TryLock()
	if err != nil {
		t.Fatalf("lock1 TryLock failed: %v", err)
	}
	if !locked {
		t.Fatal("expected lock1 TryLock to succeed")
	}
	defer lock1.Unlock()

	// Second attempt should fail (non-blocking)
	lock2 := NewIndexLock(indexPath, "sqlite", "")
	locked2, err := lock2.TryLock()
	if err != nil {
		t.Fatalf("lock2 TryLock failed: %v", err)
	}
	if locked2 {
		t.Fatal("expected lock2 TryLock to fail due to contention")
		lock2.Unlock()
	}
}

func TestIndexLock_Lock_Timeout(t *testing.T) {
	tmpDir := t.TempDir()
	indexPath := filepath.Join(tmpDir, "semantic.db")

	// First lock holder
	lock1 := NewIndexLock(indexPath, "sqlite", "")
	if err := lock1.Lock(5 * time.Second); err != nil {
		t.Fatalf("lock1 Lock failed: %v", err)
	}
	defer lock1.Unlock()

	// Second attempt should timeout
	lock2 := NewIndexLock(indexPath, "sqlite", "")
	start := time.Now()
	err := lock2.Lock(1 * time.Second)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected lock2 Lock to fail due to timeout")
		lock2.Unlock()
	}

	// Should have waited approximately 1 second
	if elapsed < 800*time.Millisecond {
		t.Errorf("expected to wait ~1s, only waited %v", elapsed)
	}
}

func TestIndexLock_ReusableAfterUnlock(t *testing.T) {
	tmpDir := t.TempDir()
	indexPath := filepath.Join(tmpDir, "semantic.db")

	lock := NewIndexLock(indexPath, "sqlite", "")

	// Lock, unlock, lock again
	if err := lock.Lock(5 * time.Second); err != nil {
		t.Fatalf("first Lock failed: %v", err)
	}
	if err := lock.Unlock(); err != nil {
		t.Fatalf("Unlock failed: %v", err)
	}

	locked, err := lock.TryLock()
	if err != nil {
		t.Fatalf("second TryLock failed: %v", err)
	}
	if !locked {
		t.Fatal("expected TryLock to succeed after unlock")
	}
	lock.Unlock()
}
