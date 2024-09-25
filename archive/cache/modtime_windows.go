//go:build windows
// +build windows

package cache

import (
	"os"
	"syscall"
	"time"
)

func SysModTime(stat os.FileInfo) time.Time {
	filetime := stat.Sys().(*syscall.Win32FileAttributeData).LastWriteTime
	return time.Unix(0, filetime.Nanoseconds())
}
