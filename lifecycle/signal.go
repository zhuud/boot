package lifecycle

import (
	"os"
	"os/signal"
	"syscall"
)

// Done 在 Cleanup 完成后关闭。
// 与 Listen 配合，让 main 阻塞直到清理结束：
//
//	lifecycle.Listen()
//	<-lifecycle.Done()
//
// 当框架（例如 Kratos）已经接管关闭时，直接调用 Cleanup() 即可。
func Done() <-chan struct{} {
	return done
}

// Listen 监听信号（默认 SIGINT/SIGTERM）并调用 Cleanup。
// 最多调用一次。不要与同样处理信号的 App 一起使用；改从 App 的 stop 钩子调用 Cleanup。
//
// 清理超时时，进程会用触发信号强杀，避免卡住的钩子让容器一直 terminating。
func Listen(signals ...os.Signal) {
	if len(signals) == 0 {
		signals = []os.Signal{syscall.SIGINT, syscall.SIGTERM}
	}

	listenOnce.Do(func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, signals...)
		go func() {
			sig := <-ch
			signal.Stop(ch)

			mu.Lock()
			forceSignal = sig
			mu.Unlock()

			_ = Cleanup()
		}()
	})
}
