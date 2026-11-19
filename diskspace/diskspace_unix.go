//go:build !windows

package diskspace

import "syscall"

// HasAtLeast reports whether path's filesystem has at least minBytes available.
func HasAtLeast(path string, minBytes uint64) (bool, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return false, err
	}
	available := stat.Bavail * uint64(stat.Bsize)
	return available >= minBytes, nil
}
