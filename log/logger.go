package log

import (
	"log/slog"
	"time"
)

// SetSlogDefault 按推荐默认组装 Logger，并设为 slog 包级默认。
//
// 默认 JSON格式
// 打印source
// attrs 分组
// 脱敏 password/token
// 截断 2048 字节
// Warn/Error 采样每秒先放行 1000 条再每 100 条一次。
// 根上绑定 service 与 env（空值也会写出）。
// 基座 panic 默认恢复；写出失败不回调，可用 [WithErrorFunc] 观测。
// 不默认抽取 trace，需要时传 [WithContextExtractor]。
// options 追加在默认之后：标量后赢，集合累加。
func SetSlogDefault(service, env string, options ...Option) {
	defaultOptions := []Option{
		WithFormat(FormatJSON),
		WithSource(true),
		WithAttrGroup("attrs"),
		WithRedactKey("password", "token"),
		WithTruncate(2048),
		WithSampling(SamplingConfig{
			Interval:   time.Second,
			Initial:    1000,
			Thereafter: 100,
		}),
	}
	handler := NewHandler(append(defaultOptions, options...)...)
	slog.SetDefault(slog.New(handler).With(
		slog.String("service", service),
		slog.String("env", env),
	))
}
