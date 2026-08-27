# log/extractor/otel

OpenTelemetry Trace / Span 上下文字段提取器，供 `boot/log` 的 `WithContextExtractor` 使用。
后续其它带依赖的抽取器放在 `log/extractor/<name>` 下，与本包并列。

## 安装

```bash
go get github.com/zhuud/boot/log/extractor/otel
```

## 用法

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

// 必须使用 *Context 方法，普通 Info 不会携带调用方 ctx。
logger.InfoContext(ctx, "request handled")
```

`SetSlogDefault` 不会默认挂上该抽取器，需要时同样传入 `WithContextExtractor`。

## 字段

| Key | 条件 |
|-----|------|
| `trace_id` | SpanContext 有效时输出 32 位十六进制 |
| `span_id` | SpanContext 有效时输出 16 位十六进制 |

无效 SpanContext 时返回 nil，不输出空字段。

## 测试

```bash
go test ./log/extractor/otel
```
