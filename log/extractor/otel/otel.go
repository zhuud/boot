// Package otel 为 log 提供 OpenTelemetry context 抽取器。
package otel

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel/trace"
)

// TraceAttrsFromContext 在 ctx 中存在有效 OpenTelemetry SpanContext 时抽取 trace_id 和 span_id。
// 可能被并发调用，必须只读取 ctx，不会启动或修改 span。
func TraceAttrsFromContext(ctx context.Context) []slog.Attr {
	spanContext := trace.SpanContextFromContext(ctx)
	if !spanContext.IsValid() {
		return nil
	}
	return []slog.Attr{
		slog.String("trace_id", spanContext.TraceID().String()),
		slog.String("span_id", spanContext.SpanID().String()),
	}
}
