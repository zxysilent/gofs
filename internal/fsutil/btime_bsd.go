//go:build darwin || freebsd

package fsutil

import (
	"os"
	"syscall"
	"time"
)

// createdAt 读 Stat_t.Birthtimespec（macOS 的 APFS/HFS+ 与 FreeBSD 的
// st_birthtim 在 Go 的 syscall 包里都映射成这个字段）。
func createdAt(_ string, info os.FileInfo) time.Time {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return time.Time{}
	}
	ts := st.Birthtimespec
	if ts.Sec == 0 && ts.Nsec == 0 {
		return time.Time{}
	}
	return time.Unix(ts.Sec, int64(ts.Nsec))
}
