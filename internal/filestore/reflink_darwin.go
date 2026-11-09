//go:build darwin

package filestore

import (
	"os"

	"golang.org/x/sys/unix"
)

// SYS_CLONEFILE is the BSD syscall number for clonefile(2) on XNU (Darwin).
// In Go, clonefile is invoked dynamically via unix.Clonefile without CGO.
const SYS_CLONEFILE = 532

func cloneFile(src, dst *os.File) error {
	dstPath := dst.Name()
	_ = os.Remove(dstPath)
	return unix.Clonefile(src.Name(), dstPath, 0)
}
