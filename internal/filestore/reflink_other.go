//go:build !linux && !darwin

package filestore

import "os"

func cloneFile(src, dst *os.File) error {
	return ErrUnsupported
}
