//go:build windows

package fsutil

import (
	"os"
	"syscall"
	"time"
)

// createdAt 读 Win32FileAttributeData.CreationTime（FILETIME，100ns 粒度）。
func createdAt(_ string, info os.FileInfo) time.Time {
	d, ok := info.Sys().(*syscall.Win32FileAttributeData)
	if !ok {
		return time.Time{}
	}
	ns := d.CreationTime.Nanoseconds()
	if ns <= 0 {
		return time.Time{}
	}
	return time.Unix(0, ns)
}
