//go:build !windows

package transcode

import (
	"os"
	"syscall"
)

func pause(p *os.Process) bool { return p.Signal(syscall.SIGSTOP) == nil }
func resume(p *os.Process)     { p.Signal(syscall.SIGCONT) }
