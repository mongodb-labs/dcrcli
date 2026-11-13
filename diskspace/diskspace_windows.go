//go:build windows

package diskspace

import "golang.org/x/sys/windows"

// HasAtLeast reports whether path's Windows volume has at least minBytes
// available to the account running DCRcli.
func HasAtLeast(path string, minBytes uint64) (bool, error) {
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return false, err
	}
	var available uint64
	if err := windows.GetDiskFreeSpaceEx(pathPtr, &available, nil, nil); err != nil {
		return false, err
	}
	return available >= minBytes, nil
}
