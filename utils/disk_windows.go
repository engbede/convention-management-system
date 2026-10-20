//go:build windows

package utils

// GetAvailableDiskSpace returns a dummy large capacity on non-unix systems when statfs is unavailable.
func GetAvailableDiskSpace(path string) (uint64, error) {
	return 100 * 1024 * 1024 * 1024, nil // 100 GB placeholder
}
