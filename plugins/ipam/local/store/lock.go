package store

import (
	"os"
	"path"

	"github.com/alexflint/go-filemutex"
)

// FileLock wraps os.File to be used as a lock using flock
type FileLock struct {
	f *filemutex.FileMutex
}

// NewFileLock opens file/dir at path and returns unlocked FileLock object
func NewFileLock(lockPath string) (*FileLock, error) {
	fi, err := os.Stat(lockPath)
	if err != nil {
		return nil, err
	}

	if fi.IsDir() {
		lockPath = path.Join(lockPath, "lock")
	}

	f, err := filemutex.New(lockPath)
	if err != nil {
		return nil, err
	}

	return &FileLock{f}, nil
}

func (l *FileLock) Close() error {
	return l.f.Close()
}

// Lock acquires an exclusive lock
func (l *FileLock) Lock() error {
	return l.f.Lock()
}

// Unlock releases the lock
func (l *FileLock) Unlock() error {
	return l.f.Unlock()
}

// Lock acquires a read lock
func (l *FileLock) RLock() error {
	return l.f.RLock()
}

// Unlock releases the read lock
func (l *FileLock) RUnlock() error {
	return l.f.RUnlock()
}