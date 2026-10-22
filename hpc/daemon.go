package hpc

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"
)

// ANSI sequence cleaner regex
var ansiCleaner = regexp.MustCompile(`\x1b(?:[@-Z\\-_]|\[[0-?]*[ -/]*[@-~])|\x1b\][^\x07\x1b]*[\x07\x1b\\]`)

// GetClusterPaths returns standard paths for socket, pid, and log files for a cluster.
func GetClusterPaths(cluster string) (string, string, string, error) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "", "", "", fmt.Errorf("unable to determine user home directory: %w", err)
	}

	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		return "", "", "", err
	}
	_ = os.Chmod(sshDir, 0o700)

	sockPath := filepath.Join(sshDir, fmt.Sprintf("hpc_ro_%s.sock", cluster))
	pidPath := filepath.Join(sshDir, fmt.Sprintf("hpc_ro_%s.pid", cluster))
	logPath := filepath.Join(sshDir, fmt.Sprintf("hpc_ro_%s.log", cluster))

	return sockPath, pidPath, logPath, nil
}

// Daemon manages a persistent, hardened read-only SSH PTY session for a specific cluster.
type Daemon struct {
	Cluster       string
	TargetHost    string
	TargetUser    string
	SockPath      string
	PidPath       string
	LogPath       string
	MasterPty     *os.File
	CmdProc       *exec.Cmd
	Running       bool
	LogFile       *os.File
	mu            sync.Mutex
	ptyLock       sync.Mutex
	promptRegex   *regexp.Regexp
	initTimeout   time.Duration
}

// NewDaemon initializes a new HPC daemon instance for the specified cluster.
func NewDaemon(cluster, targetUser string) (*Daemon, error) {
	clusterKey := strings.ToLower(strings.TrimSpace(cluster))
	if canon, ok := ClusterAliases[clusterKey]; ok {
		clusterKey = canon
	}

	cfg, ok := Clusters[clusterKey]
	if !ok {
		return nil, fmt.Errorf("unsupported cluster target: %s", cluster)
	}

	sockPath, pidPath, logPath, err := GetClusterPaths(clusterKey)
	if err != nil {
		return nil, err
	}

	resolvedHost := ResolveSSHHost(clusterKey)
	if targetUser == "" {
		targetUser = ResolveCandidateUser(clusterKey, "")
	}

	timeoutSec := cfg.TimeoutInit
	if timeoutSec <= 0 {
		timeoutSec = 45
	}

	return &Daemon{
		Cluster:     clusterKey,
		TargetHost:  resolvedHost,
		TargetUser:  targetUser,
		SockPath:    sockPath,
		PidPath:     pidPath,
		LogPath:     logPath,
		Running:     true,
		promptRegex: cfg.PromptRegex,
		initTimeout: time.Duration(timeoutSec) * time.Second,
	}, nil
}

// Log writes timestamped log messages to the daemon log file.
func (d *Daemon) Log(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	ts := time.Now().Format("2006-01-02 15:04:05")
	line := fmt.Sprintf("[%s] [%s] %s\n", ts, strings.ToUpper(d.Cluster), msg)

	d.mu.Lock()
	defer d.mu.Unlock()
	if d.LogFile != nil {
		_, _ = d.LogFile.WriteString(line)
		_ = d.LogFile.Sync()
	}
}

// StartSessionHeadless initializes the SSH PTY session non-interactively using ControlMaster.
func (d *Daemon) StartSessionHeadless() error {
	sshArgs := BuildSSHArgs(d.TargetHost, d.TargetUser)
	cmd := exec.Command("ssh", sshArgs...)

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return fmt.Errorf("failed to start headless SSH PTY session: %w", err)
	}

	d.MasterPty = ptmx
	d.CmdProc = cmd

	buf := make([]byte, 0, 8192)
	readBuf := make([]byte, 4096)
	startTime := time.Now()

	for time.Since(startTime) < d.initTimeout {
		_ = ptmx.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		n, rErr := ptmx.Read(readBuf)
		if n > 0 {
			buf = append(buf, readBuf[:n]...)
			cleaned := ansiCleaner.ReplaceAll(buf, []byte(""))
			if IsPromptDetected(buf, cleaned, d.promptRegex) {
				time.Sleep(300 * time.Millisecond)
				d.Log("Prompt candidate detected. Promoting and verifying session for %s...", strings.ToUpper(d.Cluster))

				// Promote session to standard POSIX /bin/sh and verify with probe token
				if err := d.promoteToPOSIXShell(); err != nil {
					d.Log("POSIX shell promotion or verification failed: %v. Continuing wait for genuine prompt...", err)
					continue
				}

				d.Log("Successfully promoted and verified session to POSIX /bin/sh for cluster %s.", strings.ToUpper(d.Cluster))
				return nil
			}
		}
		if rErr != nil && !os.IsTimeout(rErr) {
			break
		}
	}

	_ = cmd.Process.Kill()
	_ = ptmx.Close()
	return fmt.Errorf("timed out waiting for shell prompt on %s", d.Cluster)
}

// promoteToPOSIXShell replaces the user's default login shell (e.g. tcsh, csh, zsh)
// with standard POSIX /bin/sh to guarantee universal syntax compatibility.
func (d *Daemon) promoteToPOSIXShell() error {
	drainPTY(d.MasterPty)

	// Send "exec /bin/sh" to replace current shell process with standard POSIX /bin/sh
	if _, err := d.MasterPty.WriteString("exec /bin/sh\n"); err != nil {
		return fmt.Errorf("failed to write exec command: %w", err)
	}

	time.Sleep(200 * time.Millisecond)

	// Configure basic POSIX environment: set clean prompt, force C locale
	initScript := "PS1='__HPC_RO_SH__$ '; export PS1 2>/dev/null; export LC_ALL=C 2>/dev/null\n"
	if _, err := d.MasterPty.WriteString(initScript); err != nil {
		return fmt.Errorf("failed to write POSIX initialization script: %w", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Verify that the POSIX shell is actively responding to commands
	code, resp, err := d.ExecRawRemote("echo POSIX_READY", 10*time.Second)
	if err != nil || code != 0 || !strings.Contains(resp, "POSIX_READY") {
		return fmt.Errorf("POSIX shell verification failed (code=%d, resp=%q, err=%v)", code, resp, err)
	}

	return nil
}

// ExecRawRemote executes an internal command template over the persistent PTY and captures output.
func (d *Daemon) ExecRawRemote(cmd string, timeout time.Duration) (int, string, error) {
	d.ptyLock.Lock()
	defer d.ptyLock.Unlock()

	if d.MasterPty == nil {
		return 1, "", fmt.Errorf("master PTY session is not connected")
	}

	// Drain any stale output non-blockingly
	drainPTY(d.MasterPty)

	randBytes := make([]byte, 4)
	_, _ = rand.Read(randBytes)
	reqID := hex.EncodeToString(randBytes)
	startMarker := fmt.Sprintf("__HPC_RO_START_%s__", reqID)
	endMarker := fmt.Sprintf("__HPC_RO_END_%s__", reqID)

	fullCmd := fmt.Sprintf("echo __HPC_RO_START_\"\"%s__; (%s); echo __HPC_RO_END_\"\"%s__ $?\n", reqID, cmd, reqID)
	if _, err := d.MasterPty.WriteString(fullCmd); err != nil {
		d.Log("PTY write error: %v. Marking daemon as disconnected.", err)
		d.Running = false
		return 1, "", fmt.Errorf("PTY write error: %w", err)
	}

	readBuf := make([]byte, 65536)
	var raw strings.Builder
	startFound := false
	startTime := time.Now()

	watchdogDone := make(chan struct{})
	defer close(watchdogDone)
	go func() {
		select {
		case <-watchdogDone:
			return
		case <-time.After(timeout):
			if d.MasterPty != nil {
				_, _ = d.MasterPty.WriteString("\x03\n")
			}
		}
	}()

	for {
		if time.Since(startTime) > timeout {
			_, _ = d.MasterPty.WriteString("\x03\n") // Send Ctrl+C drain
			d.Log("Operation timed out after %v: %s", timeout, cmd)
			return 124, "", fmt.Errorf("operation timed out after %v", timeout)
		}

		_ = d.MasterPty.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
		n, rErr := d.MasterPty.Read(readBuf)
		if n > 0 {
			raw.Write(readBuf[:n])
		}

		text := raw.String()
		if !startFound {
			idx := strings.Index(text, startMarker)
			if idx != -1 {
				startFound = true
				nl := strings.Index(text[idx:], "\n")
				if nl != -1 {
					raw.Reset()
					raw.WriteString(text[idx+nl+1:])
				} else {
					raw.Reset()
					raw.WriteString(text[idx+len(startMarker):])
				}
			}
		}

		if startFound {
			textCurr := raw.String()
			idxEnd := strings.Index(textCurr, endMarker)
			if idxEnd != -1 {
				output := strings.TrimRight(textCurr[:idxEnd], "\r\n")
				after := strings.TrimSpace(textCurr[idxEnd+len(endMarker):])
				parts := strings.Fields(after)
				exitCode := 0
				if len(parts) > 0 {
					if code, err := strconv.Atoi(parts[0]); err == nil {
						exitCode = code
					}
				}
				return exitCode, output, nil
			}
		}

		if rErr != nil && !os.IsTimeout(rErr) {
			if rErr == io.EOF {
				d.Running = false
				return 1, "", fmt.Errorf("remote connection closed")
			}
			break
		}
	}

	return 1, "", fmt.Errorf("remote execution ended unexpectedly")
}

// ExecuteReadOnlyHandler strictly handles whitelisted read-only filesystem queries.
func (d *Daemon) ExecuteReadOnlyHandler(op ReadOnlyOp, remotePath string) (int, string, error) {
	safePath, err := SanitizeRemotePath(remotePath)
	if err != nil {
		return 13, fmt.Sprintf("READ_ONLY_OPERATION_DENIED: Invalid path (%v)", err), nil
	}

	quoted := QuotePosixPath(safePath)

	switch op {
	case OpList:
		cmd := fmt.Sprintf("if [ -d %s ]; then /bin/ls -1 -p -- %s; exit 0; else echo \"NOT_A_DIRECTORY\"; exit 1; fi", quoted, quoted)
		return d.ExecRawRemote(cmd, 30*time.Second)

	case OpStat:
		cmd := fmt.Sprintf("if [ -e %s ]; then stat -c \"%%s %%Y %%f\" -- %s 2>/dev/null || stat -f \"%%z %%m %%p\" -- %s 2>/dev/null; exit 0; else echo \"NOT_FOUND\"; exit 1; fi", quoted, quoted, quoted)
		return d.ExecRawRemote(cmd, 15*time.Second)

	case OpDownload:
		cmd := fmt.Sprintf(
			"if [ -f %s ]; then "+
				"sz=$(stat -c \"%%s\" -- %s 2>/dev/null || stat -f \"%%z\" -- %s 2>/dev/null || echo 0); "+
				"if [ \"$sz\" -le %d ]; then "+
				"(base64 -w 0 -- %s 2>/dev/null || base64 -b 0 -- %s 2>/dev/null || base64 < %s 2>/dev/null | tr -d \"\\r\\n\"); exit 0; "+
				"else echo \"ERR_FILE_TOO_LARGE\"; exit 2; fi; "+
				"else echo \"ERR_FILE_NOT_FOUND\"; exit 1; fi",
			quoted, quoted, quoted, MaxDownloadBytes, quoted, quoted, quoted,
		)
		return d.ExecRawRemote(cmd, 60*time.Second)

	default:
		return 13, fmt.Sprintf("READ_ONLY_OPERATION_DENIED: Operation %q is prohibited in read-only mode.", op), nil
	}
}

// HandleClient processes an incoming Unix domain socket connection.
func (d *Daemon) HandleClient(conn net.Conn) {
	defer conn.Close()

	if !VerifyPeerUID(conn) {
		d.Log("SECURITY ALERT: Rejected connection from unverified local process UID.")
		_, _ = conn.Write([]byte("13\nREAD_ONLY_OPERATION_DENIED: Unauthorized local process UID.\n"))
		return
	}

	readBuf := make([]byte, 65536)
	var rawData strings.Builder
	for {
		n, err := conn.Read(readBuf)
		if n > 0 {
			rawData.Write(readBuf[:n])
			if strings.Contains(rawData.String(), "\n") {
				break
			}
		}
		if err != nil {
			break
		}
	}

	payloadStr := strings.TrimSpace(rawData.String())
	if payloadStr == "" {
		return
	}

	var req Request
	if err := json.Unmarshal([]byte(payloadStr), &req); err != nil {
		d.Log("SECURITY ALERT: Blocked malformed JSON payload attempt: %s", payloadStr[:min(60, len(payloadStr))])
		_, _ = conn.Write([]byte("13\nREAD_ONLY_OPERATION_DENIED: Arbitrary command strings prohibited.\n"))
		return
	}

	if req.Protocol != ProtocolVersion {
		d.Log("SECURITY ALERT: Protocol version mismatch: %s", req.Protocol)
		resp := fmt.Sprintf("13\nREAD_ONLY_OPERATION_DENIED: Protocol mismatch (expected %s).\n", ProtocolVersion)
		_, _ = conn.Write([]byte(resp))
		return
	}

	switch req.Op {
	case OpPing:
		_, _ = conn.Write([]byte(fmt.Sprintf("0\nPONG IDLE [%s]\n", ProtocolVersion)))
		return

	case OpStatus:
		_, _ = conn.Write([]byte(fmt.Sprintf("0\nONLINE (IDLE) [READ_ONLY, %s]\n", ProtocolVersion)))
		return

	case OpStop:
		d.Log("Received stop request. Terminating daemon cleanly.")
		_, _ = conn.Write([]byte("0\nSTOPPING\n"))
		d.Running = false
		return
	}

	if !AllowedRemoteOps[req.Op] {
		d.Log("SECURITY ALERT: Denied non-whitelisted operation %q", req.Op)
		resp := fmt.Sprintf("13\nREAD_ONLY_OPERATION_DENIED: Operation %q is prohibited in read-only mode.\n", req.Op)
		_, _ = conn.Write([]byte(resp))
		return
	}

	d.Log("Executing Read-Only Op: %s on %s", req.Op, req.Path)
	exitCode, output, err := d.ExecuteReadOnlyHandler(req.Op, req.Path)
	if err != nil {
		resp := fmt.Sprintf("1\n%s\n", err.Error())
		_, _ = conn.Write([]byte(resp))
		return
	}

	resp := fmt.Sprintf("%d\n%s\n", exitCode, output)
	_, _ = conn.Write([]byte(resp))
}

// RunServer runs the Unix domain socket server with strict 0600 permissions.
func (d *Daemon) RunServer() error {
	_ = os.Remove(d.SockPath)

	logF, err := os.OpenFile(d.LogPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err == nil {
		d.LogFile = logF
		defer logF.Close()
	}

	d.Log("Read-Only Daemon starting for %s (PID=%d, Protocol=%s)...", d.Cluster, os.Getpid(), ProtocolVersion)

	if err := os.WriteFile(d.PidPath, []byte(strconv.Itoa(os.Getpid())+"\n"), 0o600); err != nil {
		d.Log("Failed to write PID file: %v", err)
	}
	defer os.Remove(d.PidPath)

	if d.MasterPty == nil {
		if err := d.StartSessionHeadless(); err != nil {
			d.Log("Failed to start headless session: %v", err)
			return err
		}
	}

	listener, err := net.Listen("unix", d.SockPath)
	if err != nil {
		d.Log("Failed to bind Unix socket: %v", err)
		return err
	}
	defer listener.Close()
	defer os.Remove(d.SockPath)

	_ = os.Chmod(d.SockPath, 0o600)
	d.Log("Listening on protected socket: %s (mode 0600)", d.SockPath)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		d.Log("Caught termination signal. Exiting.")
		d.Running = false
		_ = listener.Close()
	}()

	for d.Running {
		conn, err := listener.Accept()
		if err != nil {
			if !d.Running {
				break
			}
			time.Sleep(100 * time.Millisecond)
			continue
		}
		go d.HandleClient(conn)
	}

	d.Cleanup()
	return nil
}

// Cleanup terminates the active PTY session and removes socket/PID files.
func (d *Daemon) Cleanup() {
	d.Log("Stopping read-only daemon and cleaning up resources...")
	if d.MasterPty != nil {
		_, _ = d.MasterPty.WriteString("exit\n")
		time.Sleep(100 * time.Millisecond)
		_ = d.MasterPty.Close()
	}
	if d.CmdProc != nil && d.CmdProc.Process != nil {
		_ = d.CmdProc.Process.Kill()
	}
	_ = os.Remove(d.SockPath)
	_ = os.Remove(d.PidPath)
}
