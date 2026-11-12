// Package fsutil holds the tiny filesystem probes shared by the config and
// server packages.
package fsutil

import (
	"os"
	"time"
)

// IsFile reports whether p exists and is not a directory.
func IsFile(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

// IsDir reports whether p exists and is a directory.
func IsDir(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}

// CreatedAt returns the file creation time, or the zero time when the platform
// or the filesystem cannot report one (Linux needs statx and a filesystem that
// records btime). Callers hide the field for the zero value.
func CreatedAt(path string, info os.FileInfo) time.Time {
	if info == nil {
		return time.Time{}
	}
	return createdAt(path, info)
}
