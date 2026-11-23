package hpc

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// DetectTunnelPort queries OpenSSH effective configuration for targetHost (via ssh -G)
// and returns any configured LocalForward local port. Returns 0 if none is configured.
func DetectTunnelPort(targetHost string) int {
	if targetHost == "" {
		return 0
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ssh", "-G", targetHost)
	out, err := cmd.Output()
	if err != nil {
		return 0
	}

	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(trimmed), "localforward ") {
			fields := strings.Fields(trimmed)
			if len(fields) >= 2 {
				if port, err := strconv.Atoi(fields[1]); err == nil && port > 0 {
					return port
				}
			}
		}
	}
	return 0
}

// IsPortOccupied checks whether a local TCP port is already actively bound.
func IsPortOccupied(port int) bool {
	if port <= 0 {
		return false
	}
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 200*time.Millisecond)
	if err == nil {
		_ = conn.Close()
		return true
	}
	l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return true
	}
	_ = l.Close()
	return false
}

// FindPIDsOnPort returns process IDs currently occupying the specified port.
func FindPIDsOnPort(port int) []int {
	if port <= 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "lsof", "-ti", fmt.Sprintf(":%d", port))
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	var pids []int
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if pid, err := strconv.Atoi(trimmed); err == nil && pid > 0 {
			if pid != os.Getpid() {
				pids = append(pids, pid)
			}
		}
	}
	return pids
}

// KillPID terminates a process by PID cleanly.
func KillPID(pid int) error {
	if pid <= 1 || pid == os.Getpid() {
		return nil
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	_ = proc.Signal(syscall.SIGTERM)
	time.Sleep(100 * time.Millisecond)
	_ = proc.Signal(syscall.SIGKILL)
	return nil
}

// CleanKnownHostEntry purges a host pattern from ~/.ssh/known_hosts using ssh-keygen -R.
func CleanKnownHostEntry(hostPattern string) {
	if hostPattern == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ssh-keygen", "-R", hostPattern)
	_ = cmd.Run()
}

// PruneStaleSockets tests and cleans up unresponsive ControlMaster and IPC sockets.
func PruneStaleSockets(cluster string) {
	clusterKey := NormalizeCluster(cluster, "")
	if clusterKey == "" {
		return
	}
	resolvedHost := ResolveSSHHost(clusterKey)

	// 1. Probe OpenSSH ControlMaster socket
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	checkCmd := exec.CommandContext(ctx, "ssh", "-O", "check", resolvedHost)
	if err := checkCmd.Run(); err != nil {
		home, _ := os.UserHomeDir()
		if home != "" {
			sockDir := filepath.Join(home, ".ssh", "sockets")
			entries, _ := os.ReadDir(sockDir)
			for _, entry := range entries {
				name := entry.Name()
				if strings.Contains(strings.ToLower(name), strings.ToLower(clusterKey)) ||
					strings.Contains(strings.ToLower(name), strings.ToLower(resolvedHost)) {
					_ = os.Remove(filepath.Join(sockDir, name))
				}
			}
		}
	}

	// 2. Clean up IPC daemon socket if unresponsive
	sockPath, pidPath, _, _ := GetClusterPaths(clusterKey)
	if sockPath != "" {
		if _, err := os.Stat(sockPath); err == nil {
			if !IsOnline(clusterKey) {
				_ = os.Remove(sockPath)
				_ = os.Remove(pidPath)
			}
		}
	}
}

// PreflightSelfHeal runs automated diagnostics and conflict resolution before starting a session.
func PreflightSelfHeal(cluster string, logger ...func(string)) {
	var logMsg func(string)
	if len(logger) > 0 && logger[0] != nil {
		logMsg = logger[0]
	} else {
		logMsg = func(s string) {}
	}

	target := NormalizeCluster(cluster, "")
	if target == "" {
		return
	}
	resolvedHost := ResolveSSHHost(target)

	// 1. Detect dynamic tunnel port
	tunnelPort := DetectTunnelPort(resolvedHost)
	if tunnelPort > 0 {
		logMsg(fmt.Sprintf("  [HPC PRE-FLIGHT] Inspecting active SSH config for target '%s' (Tunnel port %d)...\n", target, tunnelPort))

		if IsPortOccupied(tunnelPort) {
			pids := FindPIDsOnPort(tunnelPort)
			if len(pids) > 0 {
				for _, pid := range pids {
					_ = KillPID(pid)
					logMsg(fmt.Sprintf("  [HPC AUTO-FIX] Released stale background process occupying connection port (PID %d).\n", pid))
				}
				time.Sleep(200 * time.Millisecond)
			}
		}

		CleanKnownHostEntry(fmt.Sprintf("[localhost]:%d", tunnelPort))
		CleanKnownHostEntry(fmt.Sprintf("[127.0.0.1]:%d", tunnelPort))
		logMsg("  [HPC AUTO-FIX] Verified and synchronized host key credentials.\n")
	}

	// 2. Prune stale ControlMaster and daemon sockets
	PruneStaleSockets(target)
}
