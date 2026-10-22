package hpc

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"verification_studio/mats"
)

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
	if canon, ok := ClusterAliases[c]; ok {
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

// LocateHpcRun locates an hpc-run executable if available in custom path or environment.
func LocateHpcRun() string {
	if env := os.Getenv("HPC_RUN_BIN"); env != "" {
		if fi, err := os.Stat(env); err == nil && !fi.IsDir() {
			return env
		}
	}

	// Check beside current executable
	if exe, err := os.Executable(); err == nil {
		appBin := filepath.Join(filepath.Dir(exe), "hpc-run")
		if fi, err := os.Stat(appBin); err == nil && !fi.IsDir() {
			return appBin
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

// IsOnline checks whether the cluster daemon is currently online via in-process socket ping.
func IsOnline(cluster string) bool {
	target := NormalizeCluster(cluster, "")
	if target == "gaea" {
		target = "gaea-c5"
	}

	online, _ := CheckStatus(target)
	return online
}

// EnsureConnection checks if target cluster is online. If offline, attempts headless background connection
// first (leveraging active SSH ControlMaster). If interactive authentication is required, launches a terminal.
func EnsureConnection(cluster string, models []mats.Model, logger ...func(string)) bool {
	var logMsg func(string)
	if len(logger) > 0 && logger[0] != nil {
		logMsg = logger[0]
	} else {
		logMsg = func(s string) { fmt.Print(s) }
	}

	target := NormalizeCluster(cluster, "")
	if target == "gaea" {
		target = "gaea-c5"
	}

	if !ValidClusters[target] {
		logMsg(fmt.Sprintf("  [HPC ERROR] Unsupported HPC cluster target: %s\n", target))
		return false
	}

	if IsOnline(target) {
		return true
	}

	suggestedUser := findSuggestedUser(target, models)

	selfExe, err := os.Executable()
	if err != nil {
		selfExe = os.Args[0]
	}

	// Step 1: Attempt headless background connection first (in case SSH ControlMaster is active)
	logMsg(fmt.Sprintf("  [HPC] Cluster '%s' is offline. Attempting automatic session connection...\n", target))
	if err := StartBackgroundWorker(target, suggestedUser); err == nil {
		tHeadless := time.Now()
		for time.Since(tHeadless) < 8*time.Second {
			if IsOnline(target) {
				logMsg(fmt.Sprintf("  [HPC] >>> Successfully connected to %s via active session!\n", target))
				return true
			}
			time.Sleep(300 * time.Millisecond)
		}
	}

	// Step 2: Headless connection did not succeed (interactive PIN/YubiKey needed)
	logMsg(fmt.Sprintf("  [HPC] Cluster '%s' requires PIN + YubiKey authentication. Launching Terminal...\n", target))
	logMsg("  [HPC] Please enter your credentials in the Terminal window to continue...\n")

	cleanup, err := launchAuthTerminal(target, suggestedUser, selfExe)
	if err != nil {
		logMsg(fmt.Sprintf("  [HPC ERROR] Failed to launch authentication terminal: %v\n", err))
		return false
	}
	defer cleanup()

	t0 := time.Now()
	for time.Since(t0) < 120*time.Second {
		if IsOnline(target) {
			logMsg(fmt.Sprintf("  [HPC] >>> Prompt detected! Successfully connected to %s.\n", target))
			return true
		}
		time.Sleep(500 * time.Millisecond)
	}

	logMsg(fmt.Sprintf("  [HPC ERROR] Authentication for cluster '%s' timed out after 120s.\n", target))
	return false
}

func findSuggestedUser(target string, models []mats.Model) string {
	for _, m := range models {
		mCluster := NormalizeCluster(m.Cluster, m.Path)
		if mCluster == target {
			if u := ResolveCandidateUser(target, m.Path); u != "" {
				return u
			}
		}
	}
	return ResolveCandidateUser(target, "")
}

// AllRadarImagesCached returns true and the total count if all radar forecast images
// for models with remote or local paths already exist locally with valid file size (>1000 bytes).
func AllRadarImagesCached(models []mats.Model, crefDir string, cases []RadarCase) (bool, int) {
	if len(cases) == 0 {
		return true, 0
	}
	var modelsWithPath []mats.Model
	for _, m := range models {
		if m.Path != "" {
			modelsWithPath = append(modelsWithPath, m)
		}
	}
	if len(modelsWithPath) == 0 {
		return true, 0
	}
	total := len(modelsWithPath) * len(cases)
	for _, m := range modelsWithPath {
		shortName := m.Name
		for _, c := range cases {
			localFilename := fmt.Sprintf("%s_%s_%sz_f%02d.png", shortName, c.Date, c.Cycle, c.Fhr)
			localPath := filepath.Join(crefDir, localFilename)
			if !isFileValid(localPath) {
				return false, total
			}
		}
	}
	return true, total
}

// DownloadRadarImages checks evaluated models for remote HPC or local paths and downloads cref images.
func DownloadRadarImages(
	models []mats.Model,
	crefDir string,
	cases []RadarCase,
	forceFetch bool,
	onProgress func(percent int, msg string),
	logger ...func(string),
) error {
	var logMsg func(string)
	if len(logger) > 0 && logger[0] != nil {
		logMsg = logger[0]
	} else {
		logMsg = func(s string) { fmt.Print(s) }
	}

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
				logMsg(fmt.Sprintf("  [CACHE HIT] Using cached: %s\n", localFilename))
			} else {
				needed = append(needed, c)
			}
		}

		if len(needed) == 0 {
			logMsg(fmt.Sprintf("  [ALL CACHED] All radar images for [%s] already available in local cache.\n", shortName))
			continue
		}

		if isRemote {
			if !IsOnline(cluster) {
				if !EnsureConnection(cluster, models, logMsg) || !IsOnline(cluster) {
					logMsg(fmt.Sprintf("  [HPC NOTICE] Cluster '%s' is offline. Skipping remote radar retrieval for [%s].\n", cluster, shortName))
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

			// Dynamically determine system prefixes (e.g. "arps" from "arps_noda", "rrfs" from "rrfs_exp")
			sysPrefixes := []string{"arps", "rrfs"}
			if p := strings.ToLower(strings.Split(shortName, "_")[0]); p != "" && p != "arps" && p != "rrfs" {
				sysPrefixes = append([]string{p}, sysPrefixes...)
			}

			var candidates []string
			for _, sys := range sysPrefixes {
				candidates = append(candidates,
					fmt.Sprintf("%s/com/%s/v2.1.4/%s.%s/%s/graphics/det/full/%s/cref_full_sfc_f%s.png", basePath, sys, sys, c.Date, c.Cycle, fhrDir, fhrStr),
					fmt.Sprintf("%s/%s.%s/%s/graphics/det/full/%s/cref_full_sfc_f%s.png", basePath, sys, c.Date, c.Cycle, fhrDir, fhrStr),
					fmt.Sprintf("%s/com/%s/v2.1.4/%s.%s/%s/det/graphics/full/%s/cref_full_sfc_f%s.png", basePath, sys, sys, c.Date, c.Cycle, fhrDir, fhrStr),
				)
			}
			candidates = append(candidates,
				fmt.Sprintf("%s/graphics/det/full/%s/cref_full_sfc_f%s.png", basePath, fhrDir, fhrStr),
				fmt.Sprintf("%s/%s/cref_full_sfc_f%s.png", basePath, fhrDir, fhrStr),
				fmt.Sprintf("%s/%s%s/cref_full_sfc_f%s.png", basePath, c.Date, c.Cycle, fhrStr),
				fmt.Sprintf("%s/cref_full_sfc_f%s.png", basePath, fhrStr),
			)

			if isRemote {
				downloaded := false
				for _, cand := range candidates {
					code, out, err := ClientSend(cluster, OpDownload, cand)
					if err == nil && code == 0 && out != "" {
						rawBytes, bErr := base64.StdEncoding.DecodeString(strings.TrimSpace(out))
						if bErr == nil && len(rawBytes) > 1000 {
							if n, wErr := SafeAtomicWrite(rawBytes, localPath); wErr == nil {
								logMsg(fmt.Sprintf("  [CACHE MISS] Downloaded fresh image: %s from %s (%d bytes)\n", localFilename, cluster, n))
								downloaded = true
								break
							}
						}
					}
				}
				if !downloaded {
					logMsg(fmt.Sprintf("  [HPC NOTICE] No remote radar image found for %s on %s:%s\n", localFilename, cluster, basePath))
				}
			} else {
				found := false
				for _, cand := range candidates {
					if isFileValid(cand) {
						if err := copyFile(cand, localPath); err == nil {
							logMsg(fmt.Sprintf("  [CACHE MISS] Copied fresh image: %s from local %s\n", localFilename, cand))
							found = true
							break
						}
					}
				}
				if !found {
					logMsg(fmt.Sprintf("  [LOCAL NOTICE] Local image not found: %s\n", localFilename))
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
