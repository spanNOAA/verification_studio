package hpc

import (
	"bufio"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"
)

// ResolveSSHHost resolves the destination SSH hostname or alias for a given cluster.
// If ~/.ssh/config contains a matching Host alias, it uses that configured alias.
// Otherwise, it falls back to the official Canonical FQDN hostname (Zero-Config fallback).
func ResolveSSHHost(cluster string) string {
	cfg, ok := Clusters[cluster]
	if !ok {
		return cluster
	}

	candidates := cfg.AliasCandidates
	if len(candidates) == 0 {
		candidates = []string{cfg.SSHHost, cluster}
	}

	home, err := os.UserHomeDir()
	if err == nil && home != "" {
		sshConfigFile := filepath.Join(home, ".ssh", "config")
		if hosts, err := parseSSHConfigHosts(sshConfigFile); err == nil {
			for _, cand := range candidates {
				for _, h := range hosts {
					if strings.EqualFold(h, cand) {
						return h
					}
				}
			}
		}
	}

	if cfg.CanonicalHost != "" {
		return cfg.CanonicalHost
	}
	return cfg.SSHHost
}

// parseSSHConfigHosts extracts all Host aliases defined in ~/.ssh/config.
func parseSSHConfigHosts(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var hosts []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		if strings.HasPrefix(strings.ToLower(line), "host ") {
			parts := strings.Fields(line[5:])
			for _, p := range parts {
				if p != "" && !strings.Contains(p, "*") && !strings.Contains(p, "?") {
					hosts = append(hosts, p)
				}
			}
		}
	}
	return hosts, scanner.Err()
}

// ResolveCandidateUser identifies a suggested username for the given cluster and model path.
// It searches in order:
// 1. HPC_<CLUSTER>_USER env var
// 2. HPC_USER env var
// 3. ~/.hpc_user_<cluster> and ~/.hpc_user files
// 4. User directive in ~/.ssh/config matching the cluster
// 5. Model filesystem path patterns
// 6. Current OS login username
func ResolveCandidateUser(cluster string, modelPath string) string {
	clusterEnv := strings.ToUpper(strings.ReplaceAll(cluster, "-", "_"))
	if envVal := strings.TrimSpace(os.Getenv("HPC_" + clusterEnv + "_USER")); envVal != "" {
		return envVal
	}
	if envVal := strings.TrimSpace(os.Getenv("HPC_USER")); envVal != "" {
		return envVal
	}

	home, _ := os.UserHomeDir()
	if home != "" {
		clusterUserFile := filepath.Join(home, ".hpc_user_"+cluster)
		if data, err := os.ReadFile(clusterUserFile); err == nil {
			if u := strings.TrimSpace(string(data)); u != "" {
				return u
			}
		}
		globalUserFile := filepath.Join(home, ".hpc_user")
		if data, err := os.ReadFile(globalUserFile); err == nil {
			if u := strings.TrimSpace(string(data)); u != "" {
				return u
			}
		}

		sshConfigFile := filepath.Join(home, ".ssh", "config")
		if u := extractUserFromSSHConfig(sshConfigFile, cluster); u != "" {
			return u
		}
	}

	if modelPath != "" {
		if inferred := InferUserFromModelPath(modelPath); inferred != "" {
			return inferred
		}
	}

	if cur, err := user.Current(); err == nil && cur.Username != "" {
		return cur.Username
	}

	return ""
}

// extractUserFromSSHConfig inspects ~/.ssh/config for a matching Host block with a User directive.
func extractUserFromSSHConfig(path string, cluster string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	cfg, ok := Clusters[cluster]
	candidates := []string{cluster}
	if ok && len(cfg.AliasCandidates) > 0 {
		candidates = cfg.AliasCandidates
	}

	scanner := bufio.NewScanner(f)
	currentMatch := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		lower := strings.ToLower(line)
		if strings.HasPrefix(lower, "host ") {
			parts := strings.Fields(line[5:])
			currentMatch = false
			for _, p := range parts {
				for _, c := range candidates {
					if strings.EqualFold(p, c) {
						currentMatch = true
						break
					}
				}
				if currentMatch {
					break
				}
			}
		} else if currentMatch && strings.HasPrefix(lower, "user ") {
			u := strings.TrimSpace(line[5:])
			if u != "" {
				return u
			}
		}
	}
	return ""
}

// InferUserFromModelPath attempts to infer a supercomputer username from scratch/gpfs path components.
func InferUserFromModelPath(path string) string {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`/gpfs/f\d+/([^/]+)/`),
		regexp.MustCompile(`/scratch\d*/RDARCH/([^/]+)/`),
		regexp.MustCompile(`/scratch\d*/([^/]+)/`),
		regexp.MustCompile(`/work\d*/noaa/([^/]+)/`),
	}

	for _, re := range patterns {
		if m := re.FindStringSubmatch(path); len(m) > 1 {
			candidate := strings.TrimSpace(m[1])
			if candidate != "" && candidate != "com" && candidate != "rrfs" && candidate != "ptmp" {
				return candidate
			}
		}
	}
	return ""
}

// SaveConfirmedUser persists the user-approved username to ~/.hpc_user_<cluster> and ~/.hpc_user.
func SaveConfirmedUser(cluster, username string) error {
	trimmed := strings.TrimSpace(username)
	if trimmed == "" {
		return nil
	}

	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return err
	}

	clusterFile := filepath.Join(home, ".hpc_user_"+cluster)
	_ = os.WriteFile(clusterFile, []byte(trimmed+"\n"), 0o600)

	globalFile := filepath.Join(home, ".hpc_user")
	_ = os.WriteFile(globalFile, []byte(trimmed+"\n"), 0o600)

	return nil
}

// BuildSSHArgs constructs standardized, hardened SSH argument list for cluster connection.
func BuildSSHArgs(targetHost, targetUser string) []string {
	home, _ := os.UserHomeDir()
	if home != "" {
		sockDir := filepath.Join(home, ".ssh", "sockets")
		_ = os.MkdirAll(sockDir, 0o700)
	}

	args := []string{
		"-t",
		"-o", "ControlMaster=auto",
		"-o", "ControlPath=~/.ssh/sockets/%r@%h-%p",
		"-o", "ControlPersist=3600",
		"-o", "ServerAliveInterval=30",
		"-o", "ServerAliveCountMax=3",
		"-o", "ConnectTimeout=15",
		"-o", "TCPKeepAlive=yes",
	}

	if targetUser != "" {
		args = append(args, "-l", targetUser)
	}

	args = append(args, targetHost)
	return args
}
