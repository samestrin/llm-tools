// Package storage provides storage backend abstractions for clarification tracking.
package storage

import (
	"errors"
	"fmt"
)

// Sentinel errors for storage operations
var (
	// ErrNotFound indicates the requested entry does not exist.
	ErrNotFound = errors.New("entry not found")

	// ErrDuplicateEntry indicates an entry with the same ID already exists.
	ErrDuplicateEntry = errors.New("duplicate entry")

	// ErrStorageClosed indicates an operation was attempted on a closed storage.
	ErrStorageClosed = errors.New("storage is closed")

	// ErrConstraintViolation indicates a data constraint was violated.
	ErrConstraintViolation = errors.New("constraint violation")

	// ErrUnsupportedBackend indicates the storage backend is not supported.
	ErrUnsupportedBackend = errors.New("unsupported storage backend")

	// ErrInvalidPath indicates an invalid file path was provided.
	ErrInvalidPath = errors.New("invalid path")

	// ErrConcurrentModification indicates the entry was modified by another process.
	ErrConcurrentModification = errors.New("concurrent modification detected")
)

// NotFoundError wraps ErrNotFound with context.
type NotFoundError struct {
	ID string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("clarification entry with ID '%s' not found", e.ID)
}

func (e *NotFoundError) Unwrap() error {
	return ErrNotFound
}

// DuplicateEntryError wraps ErrDuplicateEntry with context.
type DuplicateEntryError struct {
	ID string
}

func (e *DuplicateEntryError) Error() string {
	return fmt.Sprintf("clarification with ID '%s' already exists", e.ID)
}

func (e *DuplicateEntryError) Unwrap() error {
	return ErrDuplicateEntry
}

// ConstraintViolationError wraps ErrConstraintViolation with context.
type ConstraintViolationError struct {
	Message string
}

func (e *ConstraintViolationError) Error() string {
	return fmt.Sprintf("constraint violation: %s", e.Message)
}

func (e *ConstraintViolationError) Unwrap() error {
	return ErrConstraintViolation
}

// UnsupportedBackendError wraps ErrUnsupportedBackend with context.
type UnsupportedBackendError struct {
	Extension string
}

func (e *UnsupportedBackendError) Error() string {
	return fmt.Sprintf("extension '%s' is not supported (use .yaml, .yml, .db, .sqlite)", e.Extension)
}

func (e *UnsupportedBackendError) Unwrap() error {
	return ErrUnsupportedBackend
}
