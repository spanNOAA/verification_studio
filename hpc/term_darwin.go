//go:build darwin

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

	cmdFile := filepath.Join(authTempDir, "auth.command")
	ttyFile := filepath.Join(authTempDir, "auth.tty")

	scriptContent := fmt.Sprintf(`#!/bin/bash
CURRENT_TTY=$(tty)
echo "$CURRENT_TTY" > %q
printf "\033]0;MATS_HPC_AUTH_%s\007"

%q hpc-auth %q --user %q
STATUS=$?

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
`, ttyFile, strings.ToUpper(target), selfExe, target, suggestedUser)

	if err := os.WriteFile(cmdFile, []byte(scriptContent), 0o700); err != nil {
		_ = os.RemoveAll(authTempDir)
		return func() {}, err
	}

	if err := exec.Command("open", "-a", "Terminal", cmdFile).Run(); err != nil {
		_ = os.RemoveAll(authTempDir)
		return func() {}, err
	}

	cleanup := func() {
		closeTerminalByTTY(ttyFile)
		_ = os.RemoveAll(authTempDir)
	}

	return cleanup, nil
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
