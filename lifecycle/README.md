# lifecycle

进程退出时的分阶段清理钩子。框架无关；有 App（如 Kratos）时只调用 `Cleanup`，无 App 时用 `Listen` + `Done`。

## 安装

```bash
go get github.com/zhuud/boot/lifecycle
```

## API

```go
func Register(fn func() error, options ...Option)
func RegisterCloser(c io.Closer, options ...Option)
func Drain() Option                 // 放入 Drain 阶段（停流量）；默认同步 LIFO
func Async() Option                 // 当前阶段内并行执行

func Cleanup() error                // 执行一次清理；可重复调用
func Done() <-chan struct{}         // Cleanup 全部结束后关闭
func Listen(sigs ...os.Signal)      // 监听信号并调用 Cleanup；默认 SIGINT/SIGTERM

func SetTimeout(d time.Duration)    // Cleanup 超时强杀；默认 30s；非正数禁用
func SetPhaseDelay(d time.Duration) // 相邻非空阶段间隔；默认 2.5s；非正数禁用
```

## 用法

### 注册钩子

```go
lifecycle.Register(http.Stop, lifecycle.Drain(), lifecycle.Async())
lifecycle.Register(grpc.Stop, lifecycle.Drain(), lifecycle.Async())
lifecycle.RegisterCloser(db)                 // Cleanup 阶段，同步 LIFO
lifecycle.Register(flush, lifecycle.Async()) // Cleanup 阶段，并行
```

默认阶段是 Cleanup。`Drain()` 提前到停流量阶段；`Async()` 在同一阶段内并行。

### 框架托管（Kratos）

框架管信号；本包关闭强杀。不要调用 `Listen`。

`kratos.StopTimeout` 只约束 `server.Stop`，**不**约束 `BeforeStop` 里的 `Cleanup`。

```go
lifecycle.SetTimeout(0)
app := kratos.New(
	kratos.StopTimeout(30*time.Second),
	kratos.BeforeStop(func(context.Context) error {
		return lifecycle.Cleanup()
	}),
)
```

### 独立进程

没有框架兜底时使用 `Listen` + `Done`。正超时下 hook 卡死会强杀进程，避免容器一直 terminating。

```go
lifecycle.SetTimeout(30 * time.Second)
lifecycle.SetPhaseDelay(2500 * time.Millisecond)
lifecycle.Listen()
<-lifecycle.Done()
```

## Cleanup 顺序

```text
Drain（async 并行 + sync LIFO）
  ↓ 若后续仍有非空阶段：等待 phaseDelay（默认 2.5s）
Cleanup（async 并行 + sync LIFO）
  ↓
close(Done)
```

- 空阶段跳过；最后阶段结束后不再等待。
- 同步钩子按注册顺序存储，执行时反向遍历（LIFO）。
- 异步钩子与同步钩子同时启动，阶段结束前等待全部 async 完成。

## 行为边界

| 场景 | 行为 |
|------|------|
| 并发 / 重复 `Cleanup` | `sync.Once` 只跑一次；其余等待并返回相同 error |
| Cleanup 开始后的 `Register` | 立即同步执行，不再入队 |
| hook panic | `runHook` 转为 error，不影响其他 hook |
| 多 hook 错误 | `errors.Join` 聚合返回 |
| `SetTimeout` 非正数 | 禁用强杀 |
| 超时强杀 | Unix：用触发信号（`Listen` 记录；直接 `Cleanup` 默认 SIGTERM）杀进程；Windows：`os.Exit(1)`；其余平台（plan9/wasm）非正式支持，回退 `os.Exit(1)` 以便编译 |
| 超时强杀后 `Done` | 进程已退出，不会 close |
| 有 App 管信号 | 禁止 `Listen`，由 App 调用 `Cleanup` |

## 文件

```text
lifecycle.go          # Option、注册、Cleanup、超时/阶段配置、阶段执行
signal.go             # Listen、Done
forcequit_unix.go     # Unix 超时强杀（//go:build unix）
forcequit_windows.go  # Windows 超时强杀（//go:build windows）
forcequit_other.go    # 其余平台回退 os.Exit（//go:build !unix && !windows）
lifecycle_test.go
signal_unix_test.go        # Listen 子进程信号测试（//go:build unix）
lifecycle_bench_test.go
```

## 测试

```bash
go test ./lifecycle
go test -race ./lifecycle
go test -bench=. -benchmem ./lifecycle
```
