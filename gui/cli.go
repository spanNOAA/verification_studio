package main

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"verification_studio/hpc"
)

func printUsage() {
	fmt.Print(`==================================================================
 NOAA GSL Verification Studio v1.0.0 (Pure Go Universal Edition)
==================================================================
A native, lightweight numerical weather prediction model verification
and presentation generator suite.

Usage:
  verification_studio                  Launch native macOS desktop GUI
  verification_studio hpc-run <args>   Run built-in hardened NOAA HPC read-only tool
  verification_studio hpc-auth <clstr> Authenticate (PIN + YubiKey) for target cluster
  verification_studio --help           Show this help message
  verification_studio --version        Show version information
`)
}

func runCLI(args []string) bool {
	// 1. Multi-call invocation when binary is invoked directly as hpc-run
	baseName := filepath.Base(args[0])
	if baseName == "hpc-run" {
		return runHPCCLI(args[1:])
	}

	if len(args) <= 1 {
		return false
	}

	firstArg := args[1]
	// Ignore macOS Finder process serial number flag (e.g. -psn_0_12345)
	if strings.HasPrefix(firstArg, "-psn") {
		return false
	}

	// 2. Subcommands for NOAA HPC read-only operations
	if firstArg == "hpc-auth" {
		return runHPCAuth(args[2:])
	}
	if firstArg == "hpc-worker" {
		return runHPCWorker(args[2:])
	}
	if firstArg == "hpc-run" {
		return runHPCCLI(args[2:])
	}

	if firstArg == "--help" || firstArg == "-h" || firstArg == "help" {
		printUsage()
		os.Exit(0)
	}

	if firstArg == "--version" || firstArg == "-v" || firstArg == "version" {
		fmt.Println("NOAA GSL Verification Studio v1.0.0 (Pure Go Edition)")
		os.Exit(0)
	}

	// Default fallback to launch native Desktop GUI
	return false
}

func printHPCUsage() {
	fmt.Print(`==================================================================
 hpc-run: Hardened Read-Only Multi-Cluster SSH Access Tool (V1)
==================================================================
Protocol: RO-v1 (Native Pure Go Edition)

Supported Clusters:
`)
	for c, info := range hpc.Clusters {
		online, status := hpc.CheckStatus(c)
		mark := "○"
		if online {
			mark = "●"
		}
		fmt.Printf("  %s %-10s - %s [%s]\n", mark, c, info.Description, status)
	}

	fmt.Print(`
Permitted Structured Read-Only Operations (V1):
  verification_studio hpc-run <cluster> list <remote_dir>
      List remote directory entries (non-recursive)
  verification_studio hpc-run <cluster> stat <remote_path>
      Query remote file size and metadata
  verification_studio hpc-run <cluster> download <remote_path> <local_path>
      Download single remote file safely to local path (Max 50 MB)

Connection Management:
  verification_studio hpc-run --list                 List all supported clusters and live status
  verification_studio hpc-run --status [cluster|all] Check daemon status
  verification_studio hpc-run --start <cluster>      Authenticate (PIN + YubiKey) and start daemon
  verification_studio hpc-run --stop [cluster|all]   Stop background session daemon

Security Boundary:
  Arbitrary remote command execution, upload, delete, file modification,
  and job submission are permanently disabled by the read-only daemon.
`)
}

func runHPCAuth(args []string) bool {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: verification_studio hpc-auth <cluster> [--user <username>]")
		os.Exit(1)
	}
	cluster := args[0]
	candidateUser := ""
	for i := 1; i < len(args); i++ {
		if args[i] == "--user" && i+1 < len(args) {
			candidateUser = args[i+1]
			i++
		}
	}
	if err := hpc.StartInteractiveAuth(cluster, candidateUser); err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
	return true
}

func runHPCWorker(args []string) bool {
	if len(args) == 0 {
		os.Exit(1)
	}
	cluster := args[0]
	targetUser := ""
	for i := 1; i < len(args); i++ {
		if args[i] == "--user" && i+1 < len(args) {
			targetUser = args[i+1]
			i++
		}
	}
	d, err := hpc.NewDaemon(cluster, targetUser)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] %v\n", err)
		os.Exit(1)
	}
	if err := d.RunServer(); err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
	return true
}

func runHPCCLI(args []string) bool {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "--list" {
		printHPCUsage()
		os.Exit(0)
	}

	cmd := args[0]
	switch cmd {
	case "--status":
		rawTarget := "all"
		if len(args) > 1 {
			rawTarget = strings.ToLower(args[1])
		}
		if rawTarget == "all" {
			for c := range hpc.Clusters {
				_, status := hpc.CheckStatus(c)
				fmt.Printf("%-10s: %s\n", c, status)
			}
		} else {
			target := hpc.NormalizeCluster(rawTarget, "")
			if target == "" {
				fmt.Fprintf(os.Stderr, "Unknown cluster: %s\n", rawTarget)
				os.Exit(1)
			}
			_, status := hpc.CheckStatus(target)
			fmt.Printf("%s: %s\n", target, status)
		}
		os.Exit(0)

	case "--start":
		if len(args) < 2 {
			fmt.Println("Please specify a cluster to start (e.g. verification_studio hpc-run --start gaea-c6)")
			os.Exit(1)
		}
		target := hpc.NormalizeCluster(args[1], "")
		if target == "" {
			fmt.Fprintf(os.Stderr, "Unknown cluster: %s\n", args[1])
			os.Exit(1)
		}
		if err := hpc.StartInteractiveAuth(target, ""); err != nil {
			fmt.Fprintf(os.Stderr, "[ERROR] %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)

	case "--stop":
		rawTarget := "all"
		if len(args) > 1 {
			rawTarget = strings.ToLower(args[1])
		}
		if rawTarget == "all" {
			hpc.DisconnectAll()
			fmt.Println("Stopped all background read-only daemons.")
		} else {
			target := hpc.NormalizeCluster(rawTarget, "")
			if target == "" {
				fmt.Fprintf(os.Stderr, "Unknown cluster: %s\n", rawTarget)
				os.Exit(1)
			}
			_ = hpc.StopDaemon(target)
			fmt.Printf("[%s] Stopped read-only daemon.\n", strings.ToUpper(target))
		}
		os.Exit(0)
	}

	cluster := hpc.NormalizeCluster(cmd, "")
	if cluster == "" {
		fmt.Fprintf(os.Stderr, "Error: Unknown cluster or option %q.\n", cmd)
		printHPCUsage()
		os.Exit(1)
	}

	subArgs := args[1:]
	if len(subArgs) == 0 {
		fmt.Fprintf(os.Stderr, "Error: Missing read-only operation for cluster %q.\nAllowed operations: list, stat, download\n", cluster)
		os.Exit(1)
	}

	op := strings.ToLower(subArgs[0])
	switch op {
	case "list":
		if len(subArgs) < 2 {
			fmt.Fprintln(os.Stderr, "Error: 'list' requires a remote path argument.")
			os.Exit(1)
		}
		code, out, err := hpc.ClientSend(cluster, hpc.OpList, subArgs[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		if code == 0 {
			fmt.Println(out)
			os.Exit(0)
		} else {
			fmt.Fprintf(os.Stderr, "Error: %s\n", out)
			os.Exit(code)
		}

	case "stat":
		if len(subArgs) < 2 {
			fmt.Fprintln(os.Stderr, "Error: 'stat' requires a remote path argument.")
			os.Exit(1)
		}
		code, out, err := hpc.ClientSend(cluster, hpc.OpStat, subArgs[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		if code == 0 {
			fmt.Println(out)
			os.Exit(0)
		} else {
			fmt.Fprintf(os.Stderr, "Error: %s\n", out)
			os.Exit(code)
		}

	case "download":
		if len(subArgs) < 3 {
			fmt.Fprintln(os.Stderr, "Error: 'download' requires <remote_path> and <local_path> arguments.")
			os.Exit(1)
		}
		remotePath := subArgs[1]
		localPath := subArgs[2]
		code, out, err := hpc.ClientSend(cluster, hpc.OpDownload, remotePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Download failed: %v\n", err)
			os.Exit(1)
		}
		if code != 0 {
			fmt.Fprintf(os.Stderr, "Download failed: %s\n", out)
			os.Exit(code)
		}
		rawBytes, bErr := base64.StdEncoding.DecodeString(strings.TrimSpace(out))
		if bErr != nil {
			fmt.Fprintf(os.Stderr, "Failed to decode remote payload: %v\n", bErr)
			os.Exit(1)
		}
		n, wErr := hpc.SafeAtomicWrite(rawBytes, localPath)
		if wErr != nil {
			fmt.Fprintf(os.Stderr, "Failed to save local file: %v\n", wErr)
			os.Exit(1)
		}
		fmt.Printf("Successfully downloaded %s to %s (%d bytes).\n", remotePath, localPath, n)
		os.Exit(0)

	default:
		fmt.Fprintf(os.Stderr, "\n[SECURITY BLOCKED] Arbitrary shell command execution is permanently disabled.\n")
		fmt.Fprintf(os.Stderr, "Attempted operation: %s\n", op)
		fmt.Fprintf(os.Stderr, "Permitted read-only operations:\n")
		fmt.Fprintf(os.Stderr, "  hpc-run %s list <remote_dir>\n", cluster)
		fmt.Fprintf(os.Stderr, "  hpc-run %s stat <remote_path>\n", cluster)
		fmt.Fprintf(os.Stderr, "  hpc-run %s download <remote_path> <local_path>\n\n", cluster)
		fmt.Fprintf(os.Stderr, "Error: READ_ONLY_OPERATION_DENIED\n")
		os.Exit(13)
	}
	return true
}
