//go:build linux

package hpc

import (
	"net"
	"os"
	"syscall"
)

// VerifyPeerUID inspects Linux SO_PEERCRED to ensure the connecting socket belongs to the same UID.
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
		ucred, err := syscall.GetsockoptUcred(int(fd), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
		if err == nil && ucred != nil {
			peerUID = ucred.Uid
			credsFound = true
		}
	})

	if credsFound {
		return peerUID == uint32(os.Getuid())
	}

	// Fallback to POSIX 0600 file mode permissions
	return true
}
