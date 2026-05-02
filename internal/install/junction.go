package install

import (
	"fmt"
	"os"
)

func CreateJunction(link, target string) error {
	if _, err := os.Lstat(link); err == nil {
		nativeRemoveReadOnly(link)
		if err := os.RemoveAll(link); err != nil {
			return fmt.Errorf("failed to remove existing junction %s: %w", link, err)
		}
	}

	if err := os.MkdirAll(target, 0o755); err != nil {
		return fmt.Errorf("failed to create target directory %s: %w", target, err)
	}

	if err := nativeCreateJunction(link, target); err != nil {
		return fmt.Errorf("failed to create junction %s => %s: %w", link, target, err)
	}

	nativeSetReadOnly(link)
	return nil
}

func RemoveJunction(link string) error {
	nativeRemoveReadOnly(link)
	return os.RemoveAll(link)
}

func IsJunction(path string) bool {
	fi, err := os.Lstat(path)
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeSymlink != 0
}

func RemoveReadOnly(path string) {
	nativeRemoveReadOnly(path)
}
