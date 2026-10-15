//go:build windows

package main

import (
	"fmt"
	"os/exec"
)

func prepareCmdProcGroup(cmd *exec.Cmd) {}

func killCmdProcGroup(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = exec.Command("taskkill", "/F", "/T", "/PID", fmt.Sprintf("%d", cmd.Process.Pid)).Run()
	_ = cmd.Process.Kill()
}
