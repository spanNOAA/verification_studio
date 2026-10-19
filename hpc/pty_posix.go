//go:build !windows

package hpc

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/creack/pty"
	"golang.org/x/term"
)

// StartInteractiveAuth launches an interactive terminal session to authenticate via PIN + YubiKey.
// It prompts the user for their HPC username (providing a candidate default), establishes an SSH PTY,
// forwards raw terminal keystrokes for MFA, monitors for prompt detection, and then daemonizes the session.
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

	reader := bufio.NewReader(os.Stdin)
	if candidateUser != "" {
		fmt.Printf("HPC username for [%s] (press Enter for '%s', or type a new username): ", strings.ToUpper(clusterKey), candidateUser)
		input, _ := reader.ReadString('\n')
		trimmed := strings.TrimSpace(input)
		if trimmed != "" {
			candidateUser = trimmed
		}
	} else {
		for candidateUser == "" {
			fmt.Printf("Enter your HPC username for [%s]: ", strings.ToUpper(clusterKey))
			input, _ := reader.ReadString('\n')
			candidateUser = strings.TrimSpace(input)
		}
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

	cmd := exec.Command("ssh", sshArgs...)
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return fmt.Errorf("failed to start SSH PTY: %w", err)
	}
	defer ptmx.Close()

	if term.IsTerminal(int(os.Stdin.Fd())) {
		oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
		if err == nil {
			defer term.Restore(int(os.Stdin.Fd()), oldState)
		}
		_ = pty.InheritSize(os.Stdin, ptmx)
	}

	authenticated := false
	doneChan := make(chan struct{})

	// Goroutine: Forward user input to PTY
	go func() {
		_, _ = io.Copy(ptmx, os.Stdin)
	}()

	// Read PTY output, forward to stdout, and scan for prompt
	buf := make([]byte, 0, 8192)
	readBuf := make([]byte, 4096)
	startTime := time.Now()
	maxTimeout := 120 * time.Second

	for time.Since(startTime) < maxTimeout {
		_ = ptmx.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		n, err := ptmx.Read(readBuf)
		if n > 0 {
			_, _ = os.Stdout.Write(readBuf[:n])
			buf = append(buf, readBuf[:n]...)

			cleaned := ansiCleaner.ReplaceAll(buf, []byte(""))

			// Give shell adequate initialization time once login banner is observed
			if bytes.Contains(cleaned, []byte("Last login:")) && time.Since(startTime) > 30*time.Second {
				startTime = time.Now().Add(-30 * time.Second)
			}

			if IsPromptDetected(buf, cleaned, cfg.PromptRegex) {
				time.Sleep(300 * time.Millisecond)
				authenticated = true
				break
			}
		}

		if err != nil && !os.IsTimeout(err) {
			break
		}
	}
	close(doneChan)

	if !authenticated {
		fmt.Printf("\n[%s] Authentication failed or cancelled. Exiting.\n", strings.ToUpper(clusterKey))
		_ = cmd.Process.Kill()
		return fmt.Errorf("authentication failed or cancelled")
	}

	fmt.Printf("\n\n[%s] >>> Prompt detected! Authentication successful! <<<\n", strings.ToUpper(clusterKey))
	fmt.Printf("[%s] Launching background read-only session daemon...\n", strings.ToUpper(clusterKey))

	// Spawn background worker process detached from current terminal
	if err := StartBackgroundWorker(clusterKey, candidateUser); err != nil {
		return fmt.Errorf("failed to start background daemon worker: %w", err)
	}

	// Verify that daemon socket comes online
	t0 := time.Now()
	var lastErr error
	var lastResp string
	for time.Since(t0) < 15*time.Second {
		code, resp, err := ClientSend(clusterKey, OpPing, "")
		if err == nil && code == 0 && strings.Contains(resp, "PONG") {
			fmt.Printf("[%s] Read-only session is now LIVE and READY in background!\n", strings.ToUpper(clusterKey))
			fmt.Printf("[%s] Verification Studio will now retrieve radar images automatically.\n\n", strings.ToUpper(clusterKey))
			time.Sleep(1 * time.Second)
			return nil
		}
		if err != nil {
			lastErr = err
		}
		if resp != "" {
			lastResp = resp
		}
		time.Sleep(500 * time.Millisecond)
	}

	errMsg := "timed out waiting for background daemon socket to become ready"
	if lastResp != "" {
		errMsg += fmt.Sprintf(" (daemon response: %s)", strings.TrimSpace(lastResp))
	} else if lastErr != nil {
		errMsg += fmt.Sprintf(" (connection error: %v)", lastErr)
	}
	return fmt.Errorf("%s", errMsg)
}

// StartBackgroundWorker launches a detached hpc-worker process in the background.
func StartBackgroundWorker(cluster, candidateUser string) error {
	selfExe, err := os.Executable()
	if err != nil {
		selfExe = os.Args[0]
	}

	_, _, logPath, _ := GetClusterPaths(cluster)
	logF, _ := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)

	workerCmd := exec.Command(selfExe, "hpc-worker", cluster, "--user", candidateUser)
	workerCmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if logF != nil {
		workerCmd.Stdout = logF
		workerCmd.Stderr = logF
	}

	return workerCmd.Start()
}
