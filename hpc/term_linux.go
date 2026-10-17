//go:build linux

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

	cmdFile := filepath.Join(authTempDir, "auth.sh")
	scriptContent := fmt.Sprintf(`#!/bin/bash
printf "\033]0;MATS_HPC_AUTH_%s\007"

%q hpc-auth %q --user %q
STATUS=$?

if [ $STATUS -eq 0 ]; then
    echo ""
    echo "========================================================"
    echo " [SUCCESS] Connection established! Closing window..."
    echo "========================================================"
    sleep 1
    exit 0
else
    echo ""
    echo "========================================================"
    echo " [ERROR] Authentication failed (exit code: $STATUS)."
    echo "========================================================"
    read -p "Press [Enter] to close..."
    exit $STATUS
fi
`, strings.ToUpper(target), selfExe, target, suggestedUser)

	if err := os.WriteFile(cmdFile, []byte(scriptContent), 0o755); err != nil {
		_ = os.RemoveAll(authTempDir)
		return func() {}, err
	}

	// Try standard Linux terminal emulators
	terminals := [][]string{
		{"x-terminal-emulator", "-e", cmdFile},
		{"gnome-terminal", "--", cmdFile},
		{"konsole", "-e", cmdFile},
		{"xfce4-terminal", "-e", cmdFile},
		{"alacritty", "-e", cmdFile},
		{"xterm", "-e", cmdFile},
	}

	launched := false
	for _, termCmd := range terminals {
		if path, err := exec.LookPath(termCmd[0]); err == nil {
			cmd := exec.Command(path, termCmd[1:]...)
			if err := cmd.Start(); err == nil {
				launched = true
				break
			}
		}
	}

	if !launched {
		_ = os.RemoveAll(authTempDir)
		return func() {}, fmt.Errorf("no supported terminal emulator (x-terminal-emulator, gnome-terminal, konsole, xterm) found")
	}

	cleanup := func() {
		_ = os.RemoveAll(authTempDir)
	}

	return cleanup, nil
}
