//go:build windows

package lifecycle

import (
	"os"
)

func forceQuit(sig os.Signal) {
	// 写 stderr，避开 log 默认 Logger 的锁和 SetOutput。
	_, _ = os.Stderr.WriteString("lifecycle: cleanup timed out, force quit\n")
	os.Exit(1)
}
