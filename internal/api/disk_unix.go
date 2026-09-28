//go:build !windows

package api

import "syscall"

// diskSpace liefert freie und gesamte Bytes des Dateisystems unter path (0, wenn unbekannt).
func diskSpace(path string) (free, total uint64) {
	var st syscall.Statfs_t
	if syscall.Statfs(path, &st) != nil {
		return 0, 0
	}
	return st.Bavail * uint64(st.Bsize), st.Blocks * uint64(st.Bsize)
}
