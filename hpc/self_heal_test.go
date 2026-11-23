package hpc

import (
	"net"
	"testing"
)

func TestDetectTunnelPortEmpty(t *testing.T) {
	port := DetectTunnelPort("")
	if port != 0 {
		t.Errorf("expected 0 for empty host, got %d", port)
	}
}

func TestIsPortOccupied(t *testing.T) {
	// 1. Port 0 should be false
	if IsPortOccupied(0) {
		t.Errorf("expected false for port 0")
	}

	// 2. Bind a temporary ephemeral listener
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen on ephemeral port: %v", err)
	}
	defer l.Close()

	addr := l.Addr().(*net.TCPAddr)
	occupiedPort := addr.Port

	if !IsPortOccupied(occupiedPort) {
		t.Errorf("expected port %d to be reported as occupied", occupiedPort)
	}
}

func TestFindPIDsOnPortInvalid(t *testing.T) {
	pids := FindPIDsOnPort(-1)
	if len(pids) != 0 {
		t.Errorf("expected empty pids for negative port, got %v", pids)
	}
}

func TestPreflightSelfHealSafe(t *testing.T) {
	// Verify that calling PreflightSelfHeal on an unsupported or empty target does not crash
	var logged []string
	logger := func(s string) {
		logged = append(logged, s)
	}
	PreflightSelfHeal("", logger)
	PreflightSelfHeal("non-existent-cluster-999", logger)

	if len(logged) > 0 {
		t.Logf("logged messages: %v", logged)
	}
}

func TestCleanKnownHostEntryEmpty(t *testing.T) {
	// Should not panic or fail on empty input
	CleanKnownHostEntry("")
}
