# boot

Go 代码规范见 [AGENTS.md](AGENTS.md)。本库是该规范的实现示例。

## 包

| 包 | 说明 |
|----|------|
| [lifecycle](lifecycle/) | 进程退出分阶段清理钩子 |
| [log](log/) | slog Handler 构建与装饰；打日志用 slog |
| [log/file](log/file/) | 按大小轮转的文件 Writer |
| [log/extractor/otel](log/extractor/otel/) | OpenTelemetry context attrs 抽取 |

## lifecycle 速览

```go
import "github.com/zhuud/boot/lifecycle"

lifecycle.Register(http.Stop, lifecycle.Drain(), lifecycle.Async())
lifecycle.RegisterCloser(db)
lifecycle.Register(flush, lifecycle.Async())

// 有 App：关闭强杀，由框架 stop 调用 Cleanup
lifecycle.SetTimeout(0)

// 无 App：Listen + Done；超时强杀兜底
lifecycle.Listen()
<-lifecycle.Done()
```

完整说明见 [lifecycle/README.md](lifecycle/README.md)。

## log 速览

```go
import (
	"log/slog"

	"github.com/zhuud/boot/log"
)

log.SetSlogDefault("helloworld", env)
ctx = log.ContextWithAttrs(ctx, slog.String("request_id", id))
slog.InfoContext(ctx, "user created", "user_id", userId)
```

完整说明见 [log/README.md](log/README.md)。

## 本库补充约定

通用命名、错误、并发、测试见 [AGENTS.md](AGENTS.md)。本库额外固定：

- slog 装饰链（Option 传入顺序无关）：`AttrGroup → Context → Redact → Drop → Sampling → Truncate → ErrorFunc → Base`
- 不暴露 `ReplaceAttr`；source 走 `WithSource`
- `next == nil` 时 `panic("nil log handler")`；功能关闭直接返回 `next`

## 测试

```bash
gofmt -s -w .
go vet ./...
go test ./...
go test -race ./...
go test -bench=. -benchmem ./...
```
