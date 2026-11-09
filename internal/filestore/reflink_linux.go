//go:build linux

package filestore

import (
	"os"

	"golang.org/x/sys/unix"
)

// FICLONE is the ioctl request code to create a copy-on-write clone of a file range.
// Defined in linux/fs.h as _IOW(0x94, 9, int).
const FICLONE = 0x40049409

func cloneFile(src, dst *os.File) error {
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, dst.Fd(), uintptr(FICLONE), src.Fd())
	if errno != 0 {
		return errno
	}
	return nil
}
