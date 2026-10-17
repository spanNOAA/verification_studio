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
		// Darwin struct xucred (<sys/ucred.h>):
		//   u_int   cr_version;         /* layout version (offset 0, 4 bytes) */
		//   uid_t   cr_uid;             /* effective user id (offset 4, 4 bytes) */
		//   short   cr_ngroups;         /* number of groups (offset 8, 2 bytes) */
		//   gid_t   cr_groups[NGROUPS]; /* group list (offset 12, 64 bytes) */
		type xucred struct {
			Version uint32
			UID     uint32
			NGroups int16
			_       [2]byte // 2 bytes padding to align Groups to 4-byte boundary
			Groups  [16]uint32
		}

		var cred xucred
		var size = uint32(unsafe.Sizeof(cred))
		_, _, errno := syscall.Syscall6(
			syscall.SYS_GETSOCKOPT,
			fd,
			0, // SOL_LOCAL (sys/un.h)
			1, // LOCAL_PEERCRED (sys/un.h)
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
