package hpc

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
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

// VerifyPeerUID checks if the Unix socket peer process matches the current process UID.
func VerifyPeerUID(conn net.Conn) bool {
	unixConn, ok := conn.(*net.UnixConn)
	if !ok {
		return false
	}

	rawConn, err := unixConn.SyscallConn()
	if err != nil {
		return true // Fallback to POSIX 0600 file mode permissions
	}

	var peerUID uint32
	var credsFound bool

	_ = rawConn.Control(func(fd uintptr) {
		// Darwin LOCAL_PEERCRED inspection (SOL_LOCAL = 0, LOCAL_PEERCRED = 0x001)
		type xucred struct {
			Version uint32
			Type    uint32
			NumUIDs int16
			UID     uint32
			GIDs    [16]uint32
		}

		var cred xucred
		var size = uint32(unsafe.Sizeof(cred))
		_, _, errno := syscall.Syscall6(
			syscall.SYS_GETSOCKOPT,
			fd,
			0, // SOL_LOCAL
			1, // LOCAL_PEERCRED
			uintptr(unsafe.Pointer(&cred)),
			uintptr(unsafe.Pointer(&size)),
			0,
		)
		if errno == 0 {
			peerUID = cred.UID
			credsFound = true
		}
	})

	if credsFound {
		return peerUID == uint32(os.Getuid())
	}

	// If peer credential inspection is unsupported by kernel/runtime, rely on POSIX 0600 socket mode
	return true
}
