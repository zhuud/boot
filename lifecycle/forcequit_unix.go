//go:build unix

package lifecycle

import (
	"os"
	"syscall"
)

func forceQuit(sig os.Signal) {
	// 写 stderr，避开 log 默认 Logger 的锁和 SetOutput。
	_, _ = os.Stderr.WriteString("lifecycle: cleanup timed out, force quit\n")
	if s, ok := sig.(syscall.Signal); ok {
		_ = syscall.Kill(syscall.Getpid(), s)
		return
	}
	_ = syscall.Kill(syscall.Getpid(), syscall.SIGTERM)
}
