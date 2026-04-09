//go:build !windows

package install

func hideConsole() *syscall.SysProcAttr {
	return nil
}
