//go:build !windows

package utils

import "syscall"

// GetAvailableDiskSpace returns the free bytes available on the filesystem containing path.
func GetAvailableDiskSpace(path string) (uint64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, err
	}
	// Available blocks * block size = available bytes
	return stat.Bavail * uint64(stat.Bsize), nil
}

