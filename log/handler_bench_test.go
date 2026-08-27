package log

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"
)

func BenchmarkHandler_Info(b *testing.B) {
	logger := slog.New(NewHandler(
		WithWriter(io.Discard),
		WithFormat(FormatJSON),
		WithLevel(slog.LevelInfo),
	))
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		logger.Info("ready", slog.String("k", "v"))
	}
}

func BenchmarkHandler_Decorated(b *testing.B) {
	logger := slog.New(NewHandler(
		WithWriter(io.Discard),
		WithFormat(FormatJSON),
		WithRedactKey("password"),
		WithTruncate(1024),
		WithSampling(SamplingConfig{Interval: time.Hour, Initial: 1 << 20, Thereafter: 1}),
		WithDropFunc(func(context.Context, slog.Record) bool { return false }),
	))
	ctx := ContextWithAttrs(context.Background(), slog.String("request_id", "req-1"))
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		logger.InfoContext(ctx, "login", slog.String("password", "secret"), slog.String("user", "a"))
	}
}

func BenchmarkHandler_UnchangedDecorators(b *testing.B) {
	logger := slog.New(NewHandler(
		WithWriter(io.Discard),
		WithFormat(FormatJSON),
		WithRedactKey("password", "token"),
		WithTruncate(2048),
	))
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		logger.Info("ready", slog.String("user_id", "u-1"))
	}
}

func BenchmarkTruncateUTF8(b *testing.B) {
	value := stringsRepeat("你好世界", 32)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_, _ = truncateUTF8(value, 64)
	}
}

func BenchmarkTruncateAttr_Group(b *testing.B) {
	handler := &truncateHandler{maxBytes: 64}
	unchanged := slog.Group("request",
		slog.String("method", "GET"),
		slog.String("path", "/users"),
		slog.Int("status", 200),
	)
	changed := slog.Group("request",
		slog.String("method", "GET"),
		slog.String("body", stringsRepeat("x", 128)),
		slog.Int("status", 200),
	)
	b.Run("unchanged", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			_, _, _ = handler.truncateAttr(unchanged)
		}
	})
	b.Run("changed", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			_, _, _ = handler.truncateAttr(changed)
		}
	})
}

func BenchmarkRedactAttr_Group(b *testing.B) {
	handler := &redactHandler{keys: map[string]struct{}{"password": {}}}
	unchanged := slog.Group("request",
		slog.String("method", "GET"),
		slog.String("path", "/users"),
		slog.Int("status", 200),
	)
	changed := slog.Group("request",
		slog.String("method", "GET"),
		slog.String("password", "secret"),
		slog.Int("status", 200),
	)
	b.Run("unchanged", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			_, _ = handler.redactAttr(nil, unchanged)
		}
	})
	b.Run("changed", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			_, _ = handler.redactAttr(nil, changed)
		}
	})
}

func BenchmarkContextHandler(b *testing.B) {
	ctx := ContextWithAttrs(context.Background(), slog.String("request_id", "req-1"))
	record := slog.NewRecord(time.Unix(1_700_000_000, 0), slog.LevelInfo, "handled", 0)
	record.AddAttrs(slog.String("method", "GET"))
	traceAttrs := []slog.Attr{slog.String("trace_id", "trace-1")}
	single := newContextHandler(slog.DiscardHandler, attrsFromContext)
	oneNonEmpty := newContextHandler(slog.DiscardHandler, attrsFromContext, func(context.Context) []slog.Attr { return nil })
	conflict := newContextHandler(slog.DiscardHandler, attrsFromContext, func(context.Context) []slog.Attr { return traceAttrs })
	benchmarks := []struct {
		name    string
		handler slog.Handler
	}{
		{name: "single", handler: single},
		{name: "multiple_one_non_empty", handler: oneNonEmpty},
		{name: "multiple_conflict", handler: conflict},
	}
	for _, benchmark := range benchmarks {
		b.Run(benchmark.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				_ = benchmark.handler.Handle(ctx, record)
			}
		})
	}
}

func BenchmarkHashMessage(b *testing.B) {
	message := "error connecting to database timeout"
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = hashMessage(message)
	}
}

func BenchmarkSamplingCounterNextCount(b *testing.B) {
	at := time.Unix(1_700_000_000, 0)
	interval := time.Hour
	b.Run("serial", func(b *testing.B) {
		var counter samplingCounter
		counter.nextCount(at, interval)
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			counter.nextCount(at, interval)
		}
	})
	b.Run("parallel_same_bucket", func(b *testing.B) {
		var counter samplingCounter
		counter.nextCount(at, interval)
		b.ReportAllocs()
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				counter.nextCount(at, interval)
			}
		})
	})
}

func stringsRepeat(value string, n int) string {
	out := make([]byte, 0, len(value)*n)
	for range n {
		out = append(out, value...)
	}
	return string(out)
}
