//go:build unix

package config

import (
	"fmt"
	"io/fs"
	"os"
	"syscall"
)

func checkTrust(path string, info fs.FileInfo) error {
	return checkOwnership(path, info, os.Geteuid())
}

func checkOwnership(path string, info fs.FileInfo, euid int) error {
	if info.Mode().Perm()&0o002 != 0 {
		return fmt.Errorf("%s is writable by every user", path)
	}

	stat, ok := info.Sys().(*syscall.Stat_t)
	if ok && int(stat.Uid) != euid && stat.Uid != 0 {
		return fmt.Errorf("%s belongs to user ID %d, not to the current user ID %d or root",
			path, stat.Uid, euid)
	}

	return nil
}
