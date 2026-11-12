//go:build linux

package fsutil

import (
	"os"
	"runtime"
	"syscall"
	"time"
	"unsafe"
)

// statx 的系统调用号各架构不同；表里没有的架构按「不支持创建时间」处理。
var statxSyscallNr = map[string]uintptr{
	"amd64": 332,
	"arm64": 291,
	"386":   383,
	"arm":   397,
}

const (
	// AT_FDCWD 是 -100，用 uintptr 常量表达避免无符号溢出。
	atFdcwd         = ^uintptr(99)
	statxSyncAsStat = 0x0
	statxBtime      = 0x800
)

type statxTimestamp struct {
	Sec  int64
	Nsec uint32
	_    int32
}

// statx 是内核 ABI 结构体，字段顺序照抄 linux/stat.h，不能增删。
type statx struct {
	Mask           uint32
	Blksize        uint32
	Attributes     uint64
	Nlink          uint32
	Uid            uint32
	Gid            uint32
	Mode           uint16
	_              uint16
	Ino            uint64
	Size           uint64
	Blocks         uint64
	AttributesMask uint64
	Atime          statxTimestamp
	Btime          statxTimestamp
	Ctime          statxTimestamp
	Mtime          statxTimestamp
	RdevMajor      uint32
	RdevMinor      uint32
	DevMajor       uint32
	DevMinor       uint32
	MntID          uint64
	DioMemAlign    uint32
	DioOffsetAlign uint32
	_              [12]uint64
}

// createdAt 用 statx 取 btime（创建时间）。老内核或文件系统不记录 btime 时
// （部分网络挂载就是如此）返回零值，界面据此隐藏该字段。
func createdAt(path string, info os.FileInfo) time.Time {
	nr, ok := statxSyscallNr[runtime.GOARCH]
	if !ok {
		return time.Time{}
	}
	p, err := syscall.BytePtrFromString(path)
	if err != nil {
		return time.Time{}
	}
	var stx statx
	_, _, errno := syscall.Syscall6(nr, atFdcwd, uintptr(unsafe.Pointer(p)), statxSyncAsStat, statxBtime, uintptr(unsafe.Pointer(&stx)), 0)
	if errno != 0 || stx.Mask&statxBtime == 0 {
		return time.Time{}
	}
	return time.Unix(stx.Btime.Sec, int64(stx.Btime.Nsec))
}
