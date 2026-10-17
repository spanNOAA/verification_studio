//go:build windows

package hpc

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

// StartInteractiveAuth launches an interactive SSH session for Windows environments.
func StartInteractiveAuth(cluster, candidateUser string) error {
	clusterKey := strings.ToLower(strings.TrimSpace(cluster))
	if canon, ok := ClusterAliases[clusterKey]; ok {
		clusterKey = canon
	}

	cfg, ok := Clusters[clusterKey]
	if !ok {
		return fmt.Errorf("unsupported cluster target: %s", cluster)
	}

	if candidateUser == "" {
		candidateUser = ResolveCandidateUser(clusterKey, "")
	}

	// Interactive username prompt with recommended default
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("Enter your HPC username for [%s]", strings.ToUpper(clusterKey))
	if candidateUser != "" {
		fmt.Printf(" [%s]", candidateUser)
	}
	fmt.Printf(": ")

	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input != "" {
		candidateUser = input
	}
	if candidateUser != "" {
		_ = SaveConfirmedUser(clusterKey, candidateUser)
	}

	resolvedHost := ResolveSSHHost(clusterKey)
	sshArgs := BuildSSHArgs(resolvedHost, candidateUser)

	fmt.Printf("\n========================================================\n")
	fmt.Printf(" Starting read-only session for [%s] (%s)\n", strings.ToUpper(clusterKey), cfg.Description)
	fmt.Printf(" Connecting as user: %s\n", candidateUser)
	fmt.Printf(" Please enter your PIN and touch your YubiKey if prompted.\n")
	fmt.Printf("========================================================\n\n")

	sshCmd := exec.Command("ssh", sshArgs...)
	sshCmd.Stdin = os.Stdin
	sshCmd.Stdout = os.Stdout
	sshCmd.Stderr = os.Stderr

	if err := sshCmd.Run(); err != nil {
		fmt.Printf("\n[%s] Authentication failed or cancelled: %v\n", strings.ToUpper(clusterKey), err)
		return fmt.Errorf("authentication failed or cancelled: %w", err)
	}

	fmt.Printf("\n\n[%s] >>> Authentication completed successfully! <<<\n", strings.ToUpper(clusterKey))
	fmt.Printf("[%s] Launching background read-only session daemon...\n", strings.ToUpper(clusterKey))

	selfExe, err := os.Executable()
	if err != nil {
		selfExe = os.Args[0]
	}

	_, _, logPath, _ := GetClusterPaths(clusterKey)
	logF, _ := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)

	workerCmd := exec.Command(selfExe, "hpc-worker", clusterKey, "--user", candidateUser)
	workerCmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
	if logF != nil {
		workerCmd.Stdout = logF
		workerCmd.Stderr = logF
	}

	if err := workerCmd.Start(); err != nil {
		return fmt.Errorf("failed to start background daemon worker: %w", err)
	}

	// Verify that daemon socket comes online
	t0 := time.Now()
	for time.Since(t0) < 10*time.Second {
		if code, resp, err := ClientSend(clusterKey, OpPing, ""); err == nil && code == 0 && strings.Contains(resp, "PONG") {
			fmt.Printf("[%s] Read-only session is now LIVE and READY in background!\n", strings.ToUpper(clusterKey))
			fmt.Printf("[%s] Verification Studio will now retrieve radar images automatically.\n\n", strings.ToUpper(clusterKey))
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("timed out waiting for background daemon socket to become ready")
}
