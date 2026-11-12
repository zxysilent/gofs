// Package fsutil holds the tiny filesystem probes shared by the config and
// server packages.
package fsutil

import "os"

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
