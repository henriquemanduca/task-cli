package filestore

import (
	cryptorand "crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// MaxCopySize is the maximum size (100MB) for automatic attachment duplication.
// Files larger than this threshold are linked by reference instead of duplicated,
// unless overridden by forceCopy.
const MaxCopySize int64 = 100 * 1024 * 1024

// ErrUnsupported indicates that copy-on-write (reflink) cloning is not supported
// on the current platform or filesystem.
var ErrUnsupported = errors.New("reflink unsupported")

// FileShortHash computes the SHA-256 hash of the file at filePath and returns
// its first 8 hexadecimal characters.
func FileShortHash(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file for hashing: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("failed to read file for hashing: %w", err)
	}

	fullHex := hex.EncodeToString(h.Sum(nil))
	if len(fullHex) > 8 {
		return fullHex[:8], nil
	}
	return fullHex, nil
}

// cloneFileOrCopy copies a file from srcPath to dstPath. It attempts an opportunistic
// copy-on-write clone (reflink) first, falling back to standard io.Copy streaming with
// fsync and mode 0644 if CoW is unsupported or fails.
func cloneFileOrCopy(srcPath, dstPath string) error {
	src, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer src.Close()

	dst, err := os.OpenFile(dstPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dst.Close()

	// Try copy-on-write reflink first.
	if err := cloneFile(src, dst); err == nil {
		if err := dst.Sync(); err != nil {
			return fmt.Errorf("failed to sync destination file: %w", err)
		}
		return nil
	}

	// Reflink failed or unsupported: discard any partial clone and fall back
	// to a standard streaming copy on the same descriptors.
	if err := dst.Truncate(0); err != nil {
		return fmt.Errorf("failed to truncate destination file: %w", err)
	}
	if _, err := src.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("failed to rewind source file: %w", err)
	}
	if _, err := dst.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("failed to rewind destination file: %w", err)
	}

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("failed to stream copy file: %w", err)
	}

	if err := dst.Sync(); err != nil {
		return fmt.Errorf("failed to sync destination file: %w", err)
	}

	return nil
}

// SaveAttachment manages attachment duplication into the centralized task attachment directory.
//
// Rules:
// 1. Verifies srcPath exists and is a regular file. Resolves its absolute path.
// 2. If forceLink is true, does not copy and returns (absSrcPath, false, nil).
// 3. If file size > MaxCopySize and !forceCopy, does not copy and returns (absSrcPath, false, nil).
// 4. Otherwise:
//   - Resolves TaskAttachmentDir(taskID) and creates it (0755).
//   - Computes 8-character SHA-256 hash of the source file.
//   - Constructs destination filename: <filename_stem>_<short_hash>.<ext> (or <stem>_<short_hash>).
//   - Stages safely into .tmp_<stem>_<short_hash>_<nonce>.<ext>.
//   - Copies via cloneFileOrCopy.
//   - Atomically renames temporary file to final path.
//   - Returns (finalPath, true, nil).
func SaveAttachment(srcPath string, taskID int64, forceCopy, forceLink bool) (savedPath string, copied bool, err error) {
	return saveAttachment(srcPath, taskID, forceCopy, forceLink, MaxCopySize)
}

func saveAttachment(srcPath string, taskID int64, forceCopy, forceLink bool, maxCopyThreshold int64) (savedPath string, copied bool, err error) {
	st, err := os.Stat(srcPath)
	if err != nil {
		return "", false, fmt.Errorf("source file %q not found or inaccessible: %w", srcPath, err)
	}
	if !st.Mode().IsRegular() {
		return "", false, fmt.Errorf("source path %q is not a regular file", srcPath)
	}

	absSrcPath, err := filepath.Abs(srcPath)
	if err != nil {
		return "", false, fmt.Errorf("failed to resolve absolute path for %q: %w", srcPath, err)
	}

	if forceLink {
		return absSrcPath, false, nil
	}

	if st.Size() > maxCopyThreshold && !forceCopy {
		return absSrcPath, false, nil
	}

	destDir, err := TaskAttachmentDir(taskID)
	if err != nil {
		return "", false, fmt.Errorf("failed to resolve task attachment directory: %w", err)
	}

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return "", false, fmt.Errorf("failed to create task attachment directory: %w", err)
	}

	hash, err := FileShortHash(absSrcPath)
	if err != nil {
		return "", false, err
	}

	base := filepath.Base(absSrcPath)
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	if stem == "" {
		stem = base
		ext = ""
	}

	var destName string
	if ext != "" {
		destName = fmt.Sprintf("%s_%s%s", stem, hash, ext)
	} else {
		destName = fmt.Sprintf("%s_%s", stem, hash)
	}

	nonce, err := randomNonce()
	if err != nil {
		return "", false, fmt.Errorf("failed to generate staging nonce: %w", err)
	}

	var tmpName string
	if ext != "" {
		tmpName = fmt.Sprintf(".tmp_%s_%s_%s%s", stem, hash, nonce, ext)
	} else {
		tmpName = fmt.Sprintf(".tmp_%s_%s_%s", stem, hash, nonce)
	}

	tmpPath := filepath.Join(destDir, tmpName)
	finalPath := filepath.Join(destDir, destName)

	if err := cloneFileOrCopy(absSrcPath, tmpPath); err != nil {
		_ = os.Remove(tmpPath)
		return "", false, fmt.Errorf("failed to copy attachment file: %w", err)
	}

	if err := os.Rename(tmpPath, finalPath); err != nil {
		_ = os.Remove(tmpPath)
		return "", false, fmt.Errorf("failed to commit attachment file: %w", err)
	}

	return finalPath, true, nil
}

// RemoveFile safely removes a file from disk if it exists.
// It is intended for transaction rollback when database insertion fails.
// Returns nil if filePath is empty or the file does not exist.
func RemoveFile(filePath string) error {
	if filePath == "" {
		return nil
	}
	err := os.Remove(filePath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove file %q: %w", filePath, err)
	}
	return nil
}

func randomNonce() (string, error) {
	var b [4]byte
	if _, err := cryptorand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
