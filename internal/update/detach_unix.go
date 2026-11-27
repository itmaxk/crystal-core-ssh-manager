//go:build !windows

package update

import (
	"os/exec"
	"syscall"
)

// detach отвязывает новый процесс от сессии старого (терминал, SIGHUP).
func detach(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true} }
