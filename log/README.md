# log

基于标准库 `log/slog` 的 Handler 构建与装饰：编码/级别、Context attrs、脱敏、截断、采样与输出错误观测。  
不提供包级 `Info`；打日志直接用 `*slog.Logger` / `slog`。

Go 版本与模块根目录 [go.mod](../go.mod) 一致。

## 安装

```bash
go get github.com/zhuud/boot/log
```

## API

| API | 说明 |
|-----|------|
| `NewHandler` | 构建 Text/JSON 基座并应用装饰 |
| `SetSlogDefault` | 推荐默认组装 Logger 并设为 slog 全局默认 |
| `WithWriter` / `WithFormat` / `WithLevel` | 基座选项 |
| `WithSource` | 打开基座 source |
| `WithRedactKey` | 按叶子 key 或点分组路径脱敏 |
| `WithDropFunc` / `DropFunc` | 按记录内容丢弃日志 |
| `WithContextExtractor` | 从 context 抽取 attrs |
| `ContextWithAttrs` / `AttrsFromContext` | 请求级 attrs |
| `WithErrorFunc` / `ErrorFunc` | 基座写出错误回调；未配置不调用，仍恢复 panic；最后一次生效 |
| `WithSampling` / `SamplingConfig` | Warn/Error 的 Initial/Thereafter 采样 |
| `WithTruncate` | 限制 message / string / []byte 字节数 |
| `WithAttrGroup` | 本次调用属性收入指定 group；系统字段仍在根上 |

固定 Handler 顺序（Option 传入顺序无关）：

```text
AttrGroup → Context → Redact → Drop → Sampling → Truncate → ErrorFunc → Base
```

安全边界是装饰链上的 Redact / Truncate。本包不暴露 `ReplaceAttr`：它会在编码期改写已脱敏/已截断的值。需要改内建键名或时间格式时，自行使用 `slog.NewJSONHandler` / `slog.NewTextHandler` 的 `HandlerOptions`，不要从 `NewHandler` 穿过去。

级别用标准库：`slog.LevelInfo` / `slog.LevelVar` / `level.UnmarshalText` 等。

## 用法

```go
import (
	"log/slog"

	"github.com/zhuud/boot/log"
)

log.SetSlogDefault("helloworld", env)
ctx = log.ContextWithAttrs(ctx, slog.String("request_id", id))
slog.InfoContext(ctx, "user created", "user_id", userId)
```

`SetSlogDefault` 默认 JSON、打开 source、`WithAttrGroup("attrs")`、脱敏 `password` 与 `token`、
截断 2048 字节、Warn/Error 每秒先放行 1000 条随后每 100 条放行一次，并在根上绑定
`service` 与 `env`（空字符串也会原样写出）。不会默认抽取 OpenTelemetry `trace_id` /
`span_id`；需要时把 `WithContextExtractor` 追加在后面。之后直接使用 `slog.Info` /
`slog.InfoContext`。需要改 Writer、级别等时同样把 Option 追加在后面。

### 脱敏与丢弃

`WithRedactKey` 支持叶子 key 和点分组路径；多次调用会累加；空字符串忽略。`WithDropFunc`
配置一个丢弃谓词，返回 `true` 时丢弃记录；多次配置时最后一次生效，传入
`nil` 可禁用。谓词位于日志热路径，必须轻量、并发安全且不可阻塞。

```go
logger := slog.New(log.NewHandler(
	log.WithRedactKey("password", "user.token"),
	log.WithDropFunc(func(_ context.Context, record slog.Record) bool {
		return record.Message == "health check"
	}),
))
```

`DropFunc` 在 Context attrs 合并和脱敏之后执行；它能读取本次调用与 Context
注入的 attrs，但不包含 `logger.With(...)` 预绑定的 attrs。

`NewHandler` 沿用标准库构造器风格，不返回 `error`。未知 `Format` 回退为
Text。`WithWriter(nil)` 在构造时 panic。私有装饰 Handler 在 `next == nil`
时立即 panic，与 `slog.New(nil)` 一致；显式丢弃应传入 `slog.DiscardHandler`。

### 动态级别

把同一个 `*slog.LevelVar` 传给 `WithLevel`；之后调用 `Set` 即可改最低级别，无需重建 Logger。必须传指针。

```go
var level slog.LevelVar
level.Set(slog.LevelInfo)

logger := slog.New(log.NewHandler(
	log.WithWriter(os.Stdout),
	log.WithLevel(&level),
))

level.Set(slog.LevelDebug) // 并发安全，立即生效
```

### 多路输出

Go 1.26+ 使用标准库 `slog.NewMultiHandler`。每路可以有独立的格式、级别和 Writer。

```go
logger := slog.New(slog.NewMultiHandler(
	log.NewHandler(log.WithWriter(os.Stdout)),
	log.NewHandler(
		log.WithWriter(fileWriter),
		log.WithFormat(log.FormatJSON),
	),
))
```

### Context 与 Trace / Span

```go
import (
	"log/slog"

	"github.com/zhuud/boot/log"
	"github.com/zhuud/boot/log/extractor/otel"
)

logger := slog.New(log.NewHandler(
	log.WithFormat(log.FormatJSON),
	log.WithContextExtractor(otel.TraceAttrsFromContext),
))
logger.InfoContext(ctx, "request handled") // 必须用 *Context 方法
```

`SetSlogDefault` 需要 trace 时同样传入 `WithContextExtractor`。`ContextWithAttrs`
在 ctx 为 nil 时改用 `context.Background()`，不会保留取消或 deadline。
`ContextWithAttrs` 持有传入属性切片的浅拷贝，`AttrsFromContext` 也返回浅拷贝；
调用方可以修改这两个函数边界外的切片，但属性值引用的 map、slice 或指针仍由调用
方负责。同名顶层 key（多次 `ContextWithAttrs`、多个 extractor、本次调用、
`logger.With(...)`）均按 slog 顺序写出，本层不去重；JSON 解析器通常后赢。

### 输出错误观测

`slog.Logger` 会忽略 `Handler.Handle` 的返回错误。`NewHandler` 默认只恢复基座
panic，不回调写出失败。需要观测时使用：

```go
logger := slog.New(log.NewHandler(
	log.WithWriter(writer),
	log.WithErrorFunc(func(ctx context.Context, record slog.Record, err error) {
		metrics.IncLogWriteErrors()
	}),
))
```

基座 Handler 的 panic 会转换为包含堆栈的 error。配置了 `ErrorFunc` 时再进入回调。
`ErrorFunc` 贴着 Base，只观测编码/写出失败；外层装饰器（如 `DropFunc`、Truncate、
LogValuer）里的 panic 不会被这层接住。回调收到的是已经过外层装饰的 record。
普通 Error 日志不自动采集 stack。回调可能被并发调用；不得通过同一个 Logger
写回日志，避免递归。回调自身的 panic 不捕获。

### Source

```go
logger := slog.New(log.NewHandler(
	log.WithSource(true),
))
```

`WithSource` 打开基座 source。启用后每条写出的记录都带 `slog.SourceKey`；默认关闭。最后一次配置生效。

### 调用参数分组

`WithAttrGroup("attrs")` 把本次 `Info`/`Error` 等调用参数收到 `attrs` 下；`time` / `level` / `msg`、`source`（`WithSource`）、`logger.With(...)`、Context 属性以及 `log.truncated`、`log.suppressed` 仍在根上。不要用 `Logger.WithGroup` 代替 AttrGroup：它会把 `log.truncated`、`log.suppressed` 带进 group。

```go
logger := slog.New(log.NewHandler(
	log.WithFormat(log.FormatJSON),
	log.WithAttrGroup("attrs"),
	log.WithSource(true),
)).With(slog.String("service", "helloworld"))

logger.Error("user created", "user_id", "u-1")
```

```json
{"time":"...","level":"ERROR","source":{...},"msg":"user created","service":"helloworld","attrs":{"user_id":"u-1"}}
```

### 系统保留字段与分组

`source` 由 `WithSource` 与 `time` / `level` / `msg` 一同写在根上，不受
`Logger.WithGroup` 影响。`log.truncated`、`log.suppressed` 是装饰器写入的根级
保留字段，业务不得写入同名属性；`Logger.WithGroup` 会把后两者带进当前 group。
调用参数分组用 [WithAttrGroup]；单次嵌套用行内 `slog.Group(...)`：

```go
logger.Info("request handled",
	slog.Group("request",
		slog.String("path", path),
		slog.String("body", body),
	),
)
```

### 截断 / 采样

```go
logger := slog.New(log.NewHandler(
	log.WithWriter(os.Stdout),
	log.WithFormat(log.FormatJSON),
	log.WithTruncate(1024), // UTF-8 安全截断 message / string / []byte
	log.WithSampling(log.SamplingConfig{
		Interval:   time.Second,
		Initial:    1000,
		Thereafter: 100,
	}),
))
```

发生截断时，输出记录会包含 `log.truncated=true`。未使用 `Logger.WithGroup`
时写在根上；`Logger.WithGroup` 会把它带进当前 group。

`NewHandler` 默认不启用采样。`SetSlogDefault` 使用上述错误风暴保护配置：同一
Warn/Error message 每秒前 1000 条全部输出，超过后每 100 条放行一条。

采样器只处理精确的 Warn 和 Error 级别，两级各自使用 16384 个固定 message
哈希桶，启用后每个 Handler 约占用 768 KiB。每个窗口先放行 Initial 条，之后每
Thereafter 条放行一次；零 Thereafter 会丢弃窗口内其余记录。发生哈希碰撞时，
同级别的不同 message 会共享计数。后续放行记录会携带累计的
`log.suppressed`；并发抑制与放行发生竞态时，该计数可能顺延到再下一条放行
记录，但不会被重复报告。

### 文件轮转

```go
import (
	"github.com/zhuud/boot/lifecycle"
	"github.com/zhuud/boot/log/file"
)

writer, err := file.NewWriter("/var/log/app/app.log", file.WithMaxSize(100), file.WithMaxBackups(7))
if err != nil {
	return err
}
lifecycle.RegisterCloser(writer)

logger := slog.New(log.NewHandler(
	log.WithWriter(writer),
	log.WithFormat(log.FormatJSON),
))
```

`writer.Rotate()` 可由应用自己的 SIGHUP 处理流程调用，并可与 `Write` 并发执行。

## 测试

```bash
go test ./...
go test -race ./...
go test -bench=. -benchmem ./log ./log/file ./log/extractor/otel
```

## slog 分层与调用链

标准库 `Logger` / `Record` / `Attr` / `Handler` / `handleState` 的职责、曝光面、`With`/`WithGroup` 用法与性能取舍，见 [SLOG.md](SLOG.md)。
