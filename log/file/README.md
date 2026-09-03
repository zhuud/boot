# log/file

基于 [lumberjack](https://github.com/natefinch/lumberjack) 的按大小轮转文件 Writer。

## 安装

```bash
go get github.com/zhuud/boot/log/file
```

## 用法

```go
import (
	"log/slog"

	"github.com/zhuud/boot/lifecycle"
	"github.com/zhuud/boot/log"
	"github.com/zhuud/boot/log/file"
)

writer, err := file.NewWriter(
	"/var/log/app/app.log",
	file.WithMaxSize(100),
	file.WithMaxBackups(7),
	file.WithMaxAge(30),
	file.WithCompress(true),
)
if err != nil {
	return err
}
lifecycle.RegisterCloser(writer)

handler := log.NewHandler(
	log.WithWriter(writer),
	log.WithFormat(log.FormatJSON),
)
logger := slog.New(handler)
```

## 选项

| Option | 单位 / 默认 | 说明 |
|--------|-------------|------|
| `WithMaxSize` | MB / 100 | 单文件达到该大小后轮转 |
| `WithMaxBackups` | 个数 / 0 | 0 表示不按数量删除 |
| `WithMaxAge` | 天 / 0 | 0 表示不按天数删除 |
| `WithCompress` | bool / false | 轮转后 gzip 压缩 |
| `WithLocalTime` | bool / true | 备份文件名使用本地时间 |

## 约束

- `filename` 会在扩展名前拼接主机名和进程号，例如 `app.log` 变为 `app-hostname-pid.log`；
- 不要用 `lifecycle.Async()` 关闭日志 Writer；
- 构造时会创建目录并试打开文件，目录或权限错误在 `NewWriter` 返回。

`NewWriter` 校验后返回原始 `*lumberjack.Logger`（`io.WriteCloser`，并提供并发安全的
`Rotate() error`）。包本身不注册进程信号；应用可将 SIGHUP 接入 `Rotate`，也可在
测试、运维接口或轮转策略判断中主动调用。标量 Option 最后一次配置生效。
实际写入路径以返回值的 `Filename` 为准。

## 测试

```bash
go test ./log/file
go test -race ./log/file
go test -bench=. -benchmem ./log/file
```
