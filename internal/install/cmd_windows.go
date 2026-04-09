//go:build windows

package install

import "syscall"

func hideConsole() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{HideWindow: true}
}
