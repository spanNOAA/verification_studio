package hpc

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SanitizeRemotePath validates that remote path contains no PTY control characters or linebreaks.
// Allows spaces, brackets, Unicode, and other standard POSIX path characters.
func SanitizeRemotePath(path string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", fmt.Errorf("remote path cannot be empty")
	}
	if len(trimmed) > 4096 {
		return "", fmt.Errorf("remote path exceeds maximum allowable length of 4096 bytes")
	}

	for _, ch := range trimmed {
		if ch < 32 && ch != '\t' {
			return "", fmt.Errorf("path contains illegal control character (ASCII %d)", ch)
		}
		if ch == 127 {
			return "", fmt.Errorf("path contains illegal DEL control character")
		}
	}

	return trimmed, nil
}

// QuotePosixPath wraps a safe path in single quotes with proper POSIX escaping.
func QuotePosixPath(path string) string {
	if path == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(path, "'", `'\''`) + "'"
}

// SafeAtomicWrite decodes/writes data to localPath safely using an atomic temporary file.
// Disallows writing through symbolic links and enforces the 50 MB maximum download limit.
func SafeAtomicWrite(data []byte, localPath string) (int, error) {
	if len(data) > MaxDownloadBytes {
		return 0, fmt.Errorf("payload size (%d bytes) exceeds maximum allowable limit of %d bytes", len(data), MaxDownloadBytes)
	}

	absPath, err := filepath.Abs(localPath)
	if err != nil {
		return 0, fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return 0, fmt.Errorf("failed to create target directory: %w", err)
	}

	// Reject writing through existing symbolic links
	if fi, err := os.Lstat(absPath); err == nil {
		if fi.Mode()&os.ModeSymlink != 0 {
			if err := os.Remove(absPath); err != nil {
				return 0, fmt.Errorf("refusing to overwrite symbolic link at %s: %w", absPath, err)
			}
		}
	}

	randBytes := make([]byte, 4)
	_, _ = rand.Read(randBytes)
	tmpFile := filepath.Join(dir, fmt.Sprintf(".tmp_dl_%d_%s", os.Getpid(), hex.EncodeToString(randBytes)))

	if err := os.WriteFile(tmpFile, data, 0o644); err != nil {
		return 0, fmt.Errorf("failed to write temporary file: %w", err)
	}

	if err := os.Rename(tmpFile, absPath); err != nil {
		_ = os.Remove(tmpFile)
		return 0, fmt.Errorf("failed to atomically replace target file: %w", err)
	}

	return len(data), nil
}
