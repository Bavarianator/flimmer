//go:build !windows

package optimize

import "syscall"

// diskFree liefert die freien Bytes unter path (0 = unbekannt). Gleiche Logik wie internal/api.diskSpace.
func diskFree(path string) uint64 {
	var st syscall.Statfs_t
	if syscall.Statfs(path, &st) != nil {
		return 0
	}
	return st.Bavail * uint64(st.Bsize)
}
