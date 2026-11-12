//go:build !linux && !darwin && !freebsd && !windows

package fsutil

import (
	"os"
	"time"
)

// createdAt 在拿不到创建时间的平台返回零值，界面会隐藏这一栏。
func createdAt(_ string, _ os.FileInfo) time.Time {
	return time.Time{}
}
