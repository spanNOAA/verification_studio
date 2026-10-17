package hpc

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

// ClientSend sends a structured JSON request to the cluster daemon over its protected Unix socket.
func ClientSend(cluster string, op ReadOnlyOp, path string) (int, string, error) {
	clusterKey := strings.ToLower(strings.TrimSpace(cluster))
	if canon, ok := ClusterAliases[clusterKey]; ok {
		clusterKey = canon
	}

	sockPath, _, _, err := GetClusterPaths(clusterKey)
	if err != nil {
		return 1, "", err
	}

	if _, err := os.Stat(sockPath); os.IsNotExist(err) {
		return 1, "", fmt.Errorf("read-only daemon for %s is not running", clusterKey)
	}

	timeout := 65 * time.Second
	if AllowedControlOps[op] {
		timeout = 3 * time.Second
	}

	conn, err := net.DialTimeout("unix", sockPath, timeout)
	if err != nil {
		return 1, "", fmt.Errorf("failed to connect to daemon socket: %w", err)
	}
	defer conn.Close()

	req := Request{
		Op:       op,
		Path:     path,
		Protocol: ProtocolVersion,
	}

	payload, err := json.Marshal(req)
	if err != nil {
		return 1, "", fmt.Errorf("failed to serialize request: %w", err)
	}
	payload = append(payload, '\n')

	_ = conn.SetDeadline(time.Now().Add(timeout))
	if _, err := conn.Write(payload); err != nil {
		return 1, "", fmt.Errorf("failed to send request to daemon: %w", err)
	}

	var responseBuilder strings.Builder
	readBuf := make([]byte, 65536)
	totalRead := 0

	for {
		_ = conn.SetReadDeadline(time.Now().Add(timeout))
		n, rErr := conn.Read(readBuf)
		if n > 0 {
			totalRead += n
			if totalRead > MaxDownloadBytes*2 {
				return 2, "", fmt.Errorf("response size exceeded maximum allowable bound")
			}
			responseBuilder.Write(readBuf[:n])
		}
		if rErr != nil {
			break
		}
	}

	respStr := responseBuilder.String()
	firstNL := strings.Index(respStr, "\n")
	if firstNL == -1 {
		return 0, strings.TrimSpace(respStr), nil
	}

	codeStr := strings.TrimSpace(respStr[:firstNL])
	output := strings.TrimRight(respStr[firstNL+1:], "\r\n")

	exitCode := 0
	if code, err := strconv.Atoi(codeStr); err == nil {
		exitCode = code
	}

	return exitCode, output, nil
}

// CheckStatus queries whether the read-only daemon for a cluster is online.
func CheckStatus(cluster string) (bool, string) {
	clusterKey := strings.ToLower(strings.TrimSpace(cluster))
	if canon, ok := ClusterAliases[clusterKey]; ok {
		clusterKey = canon
	}

	code, resp, err := ClientSend(clusterKey, OpStatus, "")
	if err == nil && code == 0 && strings.Contains(resp, "ONLINE") {
		return true, strings.TrimSpace(resp)
	}
	return false, "OFFLINE"
}

// StopDaemon shuts down the background daemon for a specific cluster cleanly.
func StopDaemon(cluster string) error {
	clusterKey := strings.ToLower(strings.TrimSpace(cluster))
	if canon, ok := ClusterAliases[clusterKey]; ok {
		clusterKey = canon
	}

	sockPath, pidPath, _, err := GetClusterPaths(clusterKey)
	if err != nil {
		return err
	}

	// Try sending stop control message first
	_, _, _ = ClientSend(clusterKey, OpStop, "")
	time.Sleep(200 * time.Millisecond)

	// Terminate process if still running
	if pidData, err := os.ReadFile(pidPath); err == nil {
		if pid, err := strconv.Atoi(strings.TrimSpace(string(pidData))); err == nil && pid > 0 {
			if proc, err := os.FindProcess(pid); err == nil {
				_ = proc.Kill()
			}
		}
	}

	_ = os.Remove(sockPath)
	_ = os.Remove(pidPath)
	return nil
}

// DisconnectAll gracefully disconnects all active supercomputer daemons.
// Invoked automatically upon pipeline completion to ensure zero lingering background tunnels.
func DisconnectAll() {
	for c := range Clusters {
		sockPath, _, _, err := GetClusterPaths(c)
		if err == nil {
			if _, err := os.Stat(sockPath); err == nil {
				_ = StopDaemon(c)
			}
		}
	}
}
