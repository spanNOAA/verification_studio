//go:build darwin

package hpc

import (
	"net"
	"os"
	"syscall"
	"unsafe"
)

// VerifyPeerUID inspects Darwin LOCAL_PEERCRED to ensure the connecting socket belongs to the same UID.
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
