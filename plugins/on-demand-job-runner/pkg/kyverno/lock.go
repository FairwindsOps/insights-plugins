package kyverno

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Acquire acquires the policy sync lock
func (l *PolicySyncLock) Acquire() error {
	// Create lock directory if it doesn't exist
	lockDir := filepath.Dir(l.FilePath)
	if err := os.MkdirAll(lockDir, 0755); err != nil {
		return fmt.Errorf("failed to create lock directory: %w", err)
	}

	// Check if lock file exists and is not stale
	if l.isLockStale() {
		// Remove stale lock
		os.Remove(l.FilePath)
	}

	// Try to create lock file
	file, err := os.OpenFile(l.FilePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("policy sync is already running (lock file exists: %s)", l.FilePath)
		}
		return fmt.Errorf("failed to create lock file: %w", err)
	}
	defer file.Close()

	// Write lock timestamp
	lockTime := time.Now().UTC().Format(time.RFC3339)
	if _, err := file.WriteString(lockTime); err != nil {
		os.Remove(l.FilePath)
		return fmt.Errorf("failed to write lock timestamp: %w", err)
	}

	return nil
}

// Release releases the policy sync lock
func (l *PolicySyncLock) Release() error {
	return os.Remove(l.FilePath)
}

// isLockStale checks if the lock file is stale (older than lock timeout)
func (l *PolicySyncLock) isLockStale() bool {
	info, err := os.Stat(l.FilePath)
	if err != nil {
		return false // File doesn't exist, not stale
	}

	// Check if lock is older than timeout
	return time.Since(info.ModTime()) > l.LockTimeout
}
