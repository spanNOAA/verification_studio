//go:build windows

package hpc

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func launchAuthTerminal(target, suggestedUser, selfExe string) (func(), error) {
	authTempDir, err := os.MkdirTemp("", fmt.Sprintf("verif_auth_%s_", target))
	if err != nil {
		return func() {}, err
	}

	cmdFile := filepath.Join(authTempDir, "auth.bat")
	scriptContent := fmt.Sprintf(`@echo off
title MATS_HPC_AUTH_%s
echo ========================================================
echo  NOAA HPC Read-Only Authentication: %s
echo ========================================================
"%s" hpc-auth "%s" --user "%s"
if %%ERRORLEVEL%% EQU 0 (
    echo.
    echo ========================================================
    echo  [SUCCESS] Connection established! Closing in 1s...
    echo ========================================================
    timeout /t 1 /nobreak >nul
    exit 0
) else (
    echo.
    echo ========================================================
    echo  [ERROR] Authentication failed.
    echo ========================================================
    pause
    exit /b 1
)
`, strings.ToUpper(target), strings.ToUpper(target), selfExe, target, suggestedUser)

	if err := os.WriteFile(cmdFile, []byte(scriptContent), 0o755); err != nil {
		_ = os.RemoveAll(authTempDir)
		return func() {}, err
	}

	cmd := exec.Command("cmd.exe", "/c", "start", fmt.Sprintf("MATS_HPC_AUTH_%s", strings.ToUpper(target)), cmdFile)
	if err := cmd.Run(); err != nil {
		_ = os.RemoveAll(authTempDir)
		return func() {}, fmt.Errorf("failed to launch Windows terminal: %w", err)
	}

	cleanup := func() {
		_ = os.RemoveAll(authTempDir)
	}

	return cleanup, nil
}
