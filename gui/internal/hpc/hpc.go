package hpc

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"verification_studio/internal/mats"
)

// RadarCase defines a severe convective weather case study for synoptic radar comparison.
type RadarCase struct {
	Index string `json:"index"`
	Date  string `json:"date"`
	Cycle string `json:"cycle"`
	Fhr   int    `json:"fhr"`
	Title string `json:"title"`
}

// ValidClusters lists all recognized NOAA & partner HPC clusters.
var ValidClusters = map[string]bool{
	"gaea":     true,
	"gaea-c5":  true,
	"gaea-c6":  true,
	"gaea-c7":  true,
	"hera":     true,
	"ursa":     true,
	"orion":    true,
	"hercules": true,
}

var clusterAliasMap = map[string]string{
	"gaeac5":  "gaea-c5",
	"gaeac6":  "gaea-c6",
	"gaeac7":  "gaea-c7",
	"gaea-c5": "gaea-c5",
	"gaea-c6": "gaea-c6",
	"gaea-c7": "gaea-c7",
	"c5":      "gaea-c5",
	"c6":      "gaea-c6",
	"c7":      "gaea-c7",
}

// NormalizeCluster maps cluster aliases or infers cluster from base filesystem paths.
func NormalizeCluster(cluster, path string) string {
	if cluster == "" {
		if path != "" {
			if strings.Contains(path, "/gpfs/f6") {
				return "gaea-c6"
			} else if strings.Contains(path, "/gpfs/f7") {
				return "gaea-c7"
			} else if strings.Contains(path, "/gpfs/f5") {
				return "gaea-c5"
			} else if strings.Contains(path, "/scratch1/RDARCH") || strings.Contains(path, "/scratch2") {
				return "hera"
			} else if strings.Contains(path, "/work/noaa") || strings.Contains(path, "/work2/noaa") {
				return "orion"
			}
		}
		return ""
	}

	c := strings.ToLower(strings.TrimSpace(cluster))
	if canon, ok := clusterAliasMap[c]; ok {
		return canon
	}
	if c == "gaea" {
		if strings.Contains(path, "/gpfs/f6") {
			return "gaea-c6"
		} else if strings.Contains(path, "/gpfs/f7") {
			return "gaea-c7"
		} else if strings.Contains(path, "/gpfs/f5") {
			return "gaea-c5"
		}
		return "gaea-c5"
	}
	return c
}

// LocateHpcRun locates the hpc-run executable on the system.
func LocateHpcRun() string {
	if env := os.Getenv("HPC_RUN_BIN"); env != "" {
		if fi, err := os.Stat(env); err == nil && !fi.IsDir() {
			return env
		}
	}

	// Check local user bin
	home, _ := os.UserHomeDir()
	if home != "" {
		userLocal := filepath.Join(home, ".local", "bin", "hpc-run")
		if fi, err := os.Stat(userLocal); err == nil && !fi.IsDir() {
			return userLocal
		}
	}

	// Check system PATH
	if p, err := exec.LookPath("hpc-run"); err == nil {
		return p
	}

	return "hpc-run"
}

// IsOnline checks whether the cluster daemon is currently online.
func IsOnline(cluster string) bool {
	hpcBin := LocateHpcRun()
	target := NormalizeCluster(cluster, "")
	if target == "gaea" {
		target = "gaea-c5"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, hpcBin, "--status", target)
	out, err := cmd.Output()
	if err == nil && strings.Contains(string(out), "ONLINE") {
		return true
	}
	return false
}

// EnsureConnection checks if target cluster is online. If offline, prompts the user via a terminal script.
func EnsureConnection(cluster string, models []mats.Model) bool {
	target := NormalizeCluster(cluster, "")
	if target == "gaea" {
		target = "gaea-c5"
	}

	if !ValidClusters[target] {
		fmt.Printf("[ERROR] Unsupported HPC cluster target: %s\n", target)
		return false
	}

	if IsOnline(target) {
		return true
	}

	hpcBin := LocateHpcRun()
	suggestedUser := findSuggestedUser(target, models)

	fmt.Printf("\n[HPC] Cluster '%s' is offline or requires authentication.\n", target)
	fmt.Printf("[HPC] Launching interactive Terminal for PIN + YubiKey authentication...\n")

	authTempDir, err := os.MkdirTemp("", fmt.Sprintf("verif_auth_%s_", target))
	if err != nil {
		return false
	}
	defer os.RemoveAll(authTempDir)

	statusFile := filepath.Join(authTempDir, "auth.status")
	cmdFile := filepath.Join(authTempDir, "auth.command")
	ttyFile := filepath.Join(authTempDir, "auth.tty")

	targetEnvSuffix := strings.ToUpper(strings.ReplaceAll(target, "-", "_"))

	scriptContent := fmt.Sprintf(`#!/bin/bash
unset PYTHONHOME
unset PYTHONPATH
CURRENT_TTY=$(tty)
echo "$CURRENT_TTY" > %q
printf "\033]0;MATS_HPC_AUTH_%s\007"

echo "========================================================"
echo " NOAA GSL Verification Studio"
echo " HPC Authentication for [%s]"
echo "========================================================"
echo ""

SUGGESTED_USER=%q
if [ -n "$SUGGESTED_USER" ]; then
    export HPC_%s_USER="$SUGGESTED_USER"
    export HPC_USER="$SUGGESTED_USER"
fi

%q --start %q
STATUS=$?
echo "$STATUS" > %q

if [ $STATUS -eq 0 ]; then
    echo ""
    echo "========================================================"
    echo " [SUCCESS] Connection established! Closing window..."
    echo "========================================================"
    sleep 0.5
    osascript <<EOF >/dev/null 2>&1 &
tell application "Terminal"
    repeat with w in windows
        repeat with t in tabs of w
            if tty of t is "$CURRENT_TTY" then
                if (count of tabs of w) > 1 then
                    close t saving no
                else
                    close w saving no
                end if
                exit repeat
            end if
        end repeat
    end repeat
end tell
EOF
    exit 0
else
    echo ""
    echo "========================================================"
    echo " [ERROR] Authentication failed (exit code: $STATUS)."
    echo "========================================================"
    read -p "Press [Enter] to close..."
    exit $STATUS
fi
`, ttyFile, strings.ToUpper(target), strings.ToUpper(target), suggestedUser, targetEnvSuffix, hpcBin, target, statusFile)

	if err := os.WriteFile(cmdFile, []byte(scriptContent), 0o700); err != nil {
		return false
	}

	_ = exec.Command("open", "-a", "Terminal", cmdFile).Run()

	t0 := time.Now()
	for time.Since(t0) < 60*time.Second {
		if data, err := os.ReadFile(statusFile); err == nil {
			st := strings.TrimSpace(string(data))
			if st == "0" {
				fmt.Printf("[HPC] >>> Successfully connected to %s!\n", target)
				closeTerminalByTTY(ttyFile)
				return true
			} else if st != "" {
				fmt.Printf("[HPC] Authentication failed with status %s.\n", st)
				closeTerminalByTTY(ttyFile)
				return false
			}
		}
		time.Sleep(500 * time.Millisecond)
	}

	fmt.Printf("[HPC] Authentication timed out after 60s.\n")
	closeTerminalByTTY(ttyFile)
	return false
}

func closeTerminalByTTY(ttyFile string) {
	data, err := os.ReadFile(ttyFile)
	if err != nil {
		return
	}
	targetTTY := strings.TrimSpace(string(data))
	if targetTTY == "" {
		return
	}
	script := fmt.Sprintf(`
	tell application "Terminal"
		repeat with w in windows
			repeat with t in tabs of w
				if tty of t is "%s" then
					if (count of tabs of w) > 1 then
						close t saving no
					else
						close w saving no
					end if
					exit repeat
				end if
			end repeat
		end repeat
	end tell
	`, targetTTY)
	_ = exec.Command("osascript", "-e", script).Run()
}

func findSuggestedUser(target string, models []mats.Model) string {
	clusterEnv := strings.ToUpper(strings.ReplaceAll(target, "-", "_"))
	if u := os.Getenv(fmt.Sprintf("HPC_%s_USER", clusterEnv)); u != "" {
		return strings.TrimSpace(u)
	}
	if u := os.Getenv("HPC_USER"); u != "" {
		return strings.TrimSpace(u)
	}

	home, _ := os.UserHomeDir()
	if home != "" {
		for _, fn := range []string{fmt.Sprintf(".hpc_user_%s", target), ".hpc_user"} {
			if data, err := os.ReadFile(filepath.Join(home, fn)); err == nil {
				if u := strings.TrimSpace(string(data)); u != "" {
					return u
				}
			}
		}
	}

	for _, m := range models {
		path := m.Path
		mCluster := m.Cluster
		if strings.Contains(mCluster, target) || (strings.HasPrefix(target, "gaea") && strings.HasPrefix(mCluster, "gaea")) {
			re := regexp.MustCompile(`/scratch/([^/]+)/`)
			if matches := re.FindStringSubmatch(path); len(matches) > 1 {
				return matches[1]
			}
		}
	}
	return ""
}

// DownloadRadarImages checks evaluated models for remote HPC or local paths and downloads cref images.
func DownloadRadarImages(
	models []mats.Model,
	crefDir string,
	cases []RadarCase,
	forceFetch bool,
	onProgress func(percent int, msg string),
) error {
	if len(cases) == 0 {
		if onProgress != nil {
			onProgress(100, "Done")
		}
		return nil
	}

	if err := os.MkdirAll(crefDir, 0o755); err != nil {
		return err
	}

	var modelsWithPath []mats.Model
	for _, m := range models {
		if m.Path != "" {
			modelsWithPath = append(modelsWithPath, m)
		}
	}

	if len(modelsWithPath) == 0 {
		if onProgress != nil {
			onProgress(100, "Done")
		}
		return nil
	}

	hpcBin := LocateHpcRun()
	totalItems := len(modelsWithPath) * len(cases)
	currentItem := 0

	for _, m := range modelsWithPath {
		basePath := m.Path
		shortName := m.Name
		cluster := NormalizeCluster(m.Cluster, basePath)

		isRemote := ValidClusters[cluster]

		var needed []RadarCase
		for _, c := range cases {
			localFilename := fmt.Sprintf("%s_%s_%sz_f%02d.png", shortName, c.Date, c.Cycle, c.Fhr)
			localPath := filepath.Join(crefDir, localFilename)

			if !forceFetch && isFileValid(localPath) {
				currentItem++
				if onProgress != nil {
					onProgress(int(100*float64(currentItem)/float64(totalItems)), fmt.Sprintf("Using cached %s", localFilename))
				}
				fmt.Printf("  [CACHE HIT] Using cached: %s\n", localFilename)
			} else {
				needed = append(needed, c)
			}
		}

		if len(needed) == 0 {
			fmt.Printf("  [ALL CACHED] All radar images for [%s] already available in local cache.\n", shortName)
			continue
		}

		if isRemote {
			if !IsOnline(cluster) {
				if !EnsureConnection(cluster, models) || !IsOnline(cluster) {
					fmt.Printf("  [NOTICE] HPC cluster '%s' is offline. Skipping remote radar retrieval for [%s].\n", cluster, shortName)
					currentItem += len(needed)
					if onProgress != nil {
						onProgress(int(100*float64(currentItem)/float64(totalItems)), fmt.Sprintf("Cluster %s offline", cluster))
					}
					continue
				}
			}
		}

		for _, c := range needed {
			currentItem++
			fhrStr := fmt.Sprintf("%03d", c.Fhr)
			fhrDir := fmt.Sprintf("%s%s%02d", c.Date, c.Cycle, c.Fhr)
			localFilename := fmt.Sprintf("%s_%s_%sz_f%02d.png", shortName, c.Date, c.Cycle, c.Fhr)
			localPath := filepath.Join(crefDir, localFilename)

			if onProgress != nil {
				onProgress(int(100*float64(currentItem)/float64(totalItems)), fmt.Sprintf("Downloading %s", localFilename))
			}

			candidates := []string{
				fmt.Sprintf("%s/com/rrfs/v2.1.4/rrfs.%s/%s/graphics/det/full/%s/cref_full_sfc_f%s.png", basePath, c.Date, c.Cycle, fhrDir, fhrStr),
				fmt.Sprintf("%s/rrfs.%s/%s/graphics/det/full/%s/cref_full_sfc_f%s.png", basePath, c.Date, c.Cycle, fhrDir, fhrStr),
				fmt.Sprintf("%s/graphics/det/full/%s/cref_full_sfc_f%s.png", basePath, fhrDir, fhrStr),
				fmt.Sprintf("%s/%s/cref_full_sfc_f%s.png", basePath, fhrDir, fhrStr),
				fmt.Sprintf("%s/%s%s/cref_full_sfc_f%s.png", basePath, c.Date, c.Cycle, fhrStr),
				fmt.Sprintf("%s/cref_full_sfc_f%s.png", basePath, fhrStr),
			}

			if isRemote {
				downloaded := false
				for _, cand := range candidates {
					ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
					cmd := exec.CommandContext(ctx, hpcBin, cluster, "download", cand, localPath)
					cmd.Env = append(os.Environ(), "HPC_NON_INTERACTIVE=1")
					var errBuf bytes.Buffer
					cmd.Stderr = &errBuf
					err := cmd.Run()
					cancel()

					if err == nil && isFileValid(localPath) {
						fi, _ := os.Stat(localPath)
						fmt.Printf("  [CACHE MISS] Downloaded fresh image: %s from %s (%d bytes)\n", localFilename, cluster, fi.Size())
						downloaded = true
						break
					}
				}
				if !downloaded {
					fmt.Printf("  [NOTICE] No remote radar image found for %s on %s:%s\n", localFilename, cluster, basePath)
				}
			} else {
				found := false
				for _, cand := range candidates {
					if isFileValid(cand) {
						if err := copyFile(cand, localPath); err == nil {
							fmt.Printf("  [CACHE MISS] Copied fresh image: %s from local %s\n", localFilename, cand)
							found = true
							break
						}
					}
				}
				if !found {
					fmt.Printf("  [NOTICE] Local image not found: %s\n", localFilename)
				}
			}
		}
	}

	if onProgress != nil {
		onProgress(100, "Done")
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func isFileValid(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	return info.Size() > 1000
}
