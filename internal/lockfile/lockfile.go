package lockfile

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

const defaultLockFile = ".ipfs_publisher.lock"

// Lockfile represents a process lock file
type Lockfile struct {
	path string
	file *os.File
}

// New creates a new lockfile instance
func New(baseDir string) *Lockfile {
	lockPath := filepath.Join(baseDir, defaultLockFile)
	return &Lockfile{path: lockPath}
}

// Acquire attempts to acquire the lock
func (l *Lockfile) Acquire() error {
	// Expand tilde in path
	if strings.HasPrefix(l.path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}
		l.path = filepath.Join(home, l.path[1:])
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(l.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create lock directory: %w", err)
	}

	// Check if lock file exists
	if _, err := os.Stat(l.path); err == nil {
		// Lock file exists, check if process is still running
		pid, err := l.readPID()
		if err == nil {
			if l.isProcessRunning(pid) {
				return fmt.Errorf("another instance is already running (PID: %d)", pid)
			}
			// Process not running, remove stale lock file
			if err := os.Remove(l.path); err != nil {
				return fmt.Errorf("failed to remove stale lock file: %w", err)
			}
		}
	}

	// Create lock file
	file, err := os.OpenFile(l.path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("failed to create lock file (another instance may be starting)")
		}
		return fmt.Errorf("failed to create lock file: %w", err)
	}

	l.file = file

	// Write current PID to lock file
	pid := os.Getpid()
	if _, err := file.WriteString(fmt.Sprintf("%d\n", pid)); err != nil {
		file.Close()
		os.Remove(l.path)
		return fmt.Errorf("failed to write PID to lock file: %w", err)
	}

	// Sync to disk
	if err := file.Sync(); err != nil {
		file.Close()
		os.Remove(l.path)
		return fmt.Errorf("failed to sync lock file: %w", err)
	}

	return nil
}

// Release releases the lock
func (l *Lockfile) Release() error {
	if l.file != nil {
		l.file.Close()
		l.file = nil
	}

	if err := os.Remove(l.path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove lock file: %w", err)
	}

	return nil
}

// readPID reads the PID from the lock file
func (l *Lockfile) readPID() (int, error) {
	data, err := os.ReadFile(l.path)
	if err != nil {
		return 0, err
	}

	pidStr := strings.TrimSpace(string(data))
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return 0, fmt.Errorf("invalid PID in lock file: %w", err)
	}

	return pid, nil
}

// isProcessRunning checks if a process with the given PID is running
func (l *Lockfile) isProcessRunning(pid int) bool {
	// Send signal 0 to check if process exists
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// On Unix systems, signal 0 checks process existence without actually sending a signal
	err = process.Signal(syscall.Signal(0))
	if err == nil {
		return true
	}

	// Check if error is "process finished" or "no such process"
	if err == os.ErrProcessDone || strings.Contains(err.Error(), "no such process") {
		return false
	}

	// For permission errors, assume process is running
	return true
}
