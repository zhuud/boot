// Package lifecycle 按有序阶段协调进程退出钩子。
//
// 框架托管的 stop（例如 Kratos BeforeStop）：
//
//	lifecycle.SetTimeout(0) // 关闭 lifecycle 强杀
//	app := kratos.New(
//		kratos.StopTimeout(30*time.Second), // 作用于 server.Stop
//		kratos.BeforeStop(func(context.Context) error {
//			return lifecycle.Cleanup()
//		}),
//	)
//
// 独立进程（没有框架信号处理）：
//
//	lifecycle.Register(stopTraffic, lifecycle.Drain())
//	lifecycle.RegisterCloser(db)
//	lifecycle.Register(flush, lifecycle.Async())
//	lifecycle.Listen()
//	<-lifecycle.Done()
//
// Cleanup 先跑 drain 阶段再跑 cleanup 阶段。每个阶段内，异步钩子并行执行，
// 同步钩子按 LIFO 顺序执行。正超时下，卡住的清理会强杀进程。
package lifecycle

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"syscall"
	"time"
)

type phase uint8

const (
	phaseDrain phase = iota
	phaseCleanup
	phaseCount
)

// phaseHooks 保存一个阶段内注册的同步与异步回调。
type phaseHooks struct {
	serial   []func() error
	parallel []func() error
}

func (h phaseHooks) empty() bool {
	return len(h.serial) == 0 && len(h.parallel) == 0
}

func (h phaseHooks) run() []error {
	parallelErrs := make([]error, len(h.parallel))
	var wg sync.WaitGroup
	for i, fn := range h.parallel {
		wg.Add(1)
		go func() {
			defer wg.Done()
			parallelErrs[i] = runHook(fn)
		}()
	}

	// 按 LIFO 顺序执行同步钩子。
	var errs []error
	for i := len(h.serial) - 1; i >= 0; i-- {
		if err := runHook(h.serial[i]); err != nil {
			errs = append(errs, err)
		}
	}
	wg.Wait()
	return append(errs, parallelErrs...)
}

type registerOptions struct {
	phase phase
	async bool
}

// Option 配置通过 [Register] 注册的钩子。
type Option func(*registerOptions)

// Drain 把钩子放到资源清理之前的 drain 阶段。
// 与 Async() 组合可并行 drain；默认是同步 LIFO。
func Drain() Option {
	return func(options *registerOptions) { options.phase = phaseDrain }
}

// Async 在所属阶段（Drain 或 cleanup）内并行运行该钩子。
func Async() Option {
	return func(options *registerOptions) { options.async = true }
}

var (
	mu         sync.Mutex
	registered [phaseCount]phaseHooks
	stopping   bool

	done        = make(chan struct{})
	cleanupOnce sync.Once
	cleanupErr  error

	listenOnce  sync.Once
	forceSignal os.Signal = syscall.SIGTERM

	timeout    = 30 * time.Second
	phaseDelay = 2500 * time.Millisecond
)

// SetTimeout 设置 Cleanup 运行多久后强杀进程。
// 非正 duration 禁用强杀。
//
// 强杀面向 Listen + Done 的独立进程路径：没有其他机制能结束卡在清理钩子里的进程。
// 当 Cleanup 接到框架 stop 钩子（例如 Kratos BeforeStop）时，优先 SetTimeout(0)，
// 让框架/编排器执行截止时间。
func SetTimeout(d time.Duration) {
	mu.Lock()
	timeout = d
	mu.Unlock()
}

// SetPhaseDelay 设置相邻非空关闭阶段之间的延迟。
// 非正 duration 禁用该延迟。
func SetPhaseDelay(d time.Duration) {
	if d < 0 {
		d = 0
	}
	mu.Lock()
	phaseDelay = d
	mu.Unlock()
}

// Register 追加一个清理钩子（默认同步 LIFO）。
// Cleanup 已经开始后，fn 会立即执行。
func Register(fn func() error, options ...Option) {
	if fn == nil {
		return
	}
	registration := registerOptions{phase: phaseCleanup}
	for _, option := range options {
		if option != nil {
			option(&registration)
		}
	}

	mu.Lock()
	if stopping {
		mu.Unlock()
		_ = runHook(fn)
		return
	}
	hooks := &registered[registration.phase]
	if registration.async {
		hooks.parallel = append(hooks.parallel, fn)
	} else {
		hooks.serial = append(hooks.serial, fn)
	}
	mu.Unlock()
}

// RegisterCloser 把 c.Close 注册为钩子。
func RegisterCloser(c io.Closer, options ...Option) {
	if c != nil {
		Register(c.Close, options...)
	}
}

// Cleanup 执行已注册钩子一次。重复调用是安全的。
func Cleanup() error {
	cleanupOnce.Do(func() {
		mu.Lock()
		stopping = true
		pending := registered
		registered = [phaseCount]phaseHooks{}
		forceTimeout := timeout
		delay := phaseDelay
		sig := forceSignal
		mu.Unlock()

		stop := startForceQuitTimer(forceTimeout, sig)
		defer stop()

		var errs []error
		ranPhase := false
		for _, h := range pending {
			if h.empty() {
				continue
			}
			if ranPhase && delay > 0 {
				time.Sleep(delay)
			}
			errs = append(errs, h.run()...)
			ranPhase = true
		}
		cleanupErr = errors.Join(errs...)
		close(done)
	})
	return cleanupErr
}

func runHook(fn func() error) (err error) {
	defer func() {
		if v := recover(); v != nil {
			err = fmt.Errorf("lifecycle: hook panic: %v", v)
		}
	}()
	return fn()
}

func startForceQuitTimer(d time.Duration, sig os.Signal) func() {
	if d <= 0 {
		return func() {}
	}
	timer := time.AfterFunc(d, func() { forceQuit(sig) })
	return func() { timer.Stop() }
}
