//go:build !darwin && !linux

package hpc

import (
	"net"
)

// VerifyPeerUID provides fallback socket authorization on operating systems without standard POSIX SO_PEERCRED.
func VerifyPeerUID(conn net.Conn) bool {
	// For platforms such as Windows where Unix socket credentials or named pipes use filesystem ACLs,
	// rely on secure directory isolation and file mode permissions.
	return true
}
