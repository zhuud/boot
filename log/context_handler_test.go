package log

import (
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestContextWithAttrs_Appends(t *testing.T) {
	ctx := ContextWithAttrs(context.Background(), slog.String("a", "1"))
	ctx = ContextWithAttrs(ctx, slog.String("b", "2"))
	attrs := AttrsFromContext(ctx)
	if len(attrs) != 2 {
		t.Fatalf("len(attrs) = %d; want 2", len(attrs))
	}
	if attrs[0].Key != "a" || attrs[1].Key != "b" {
		t.Fatalf("attrs keys = %q,%q; want a,b", attrs[0].Key, attrs[1].Key)
	}
}

func TestContextWithAttrs_HandlesNilAndEmptyInput(t *testing.T) {
	ctx := ContextWithAttrs(nil, slog.String("request_id", "r1")) //nolint:staticcheck // 文档约定 nil ctx 改用 Background
	if attrs := AttrsFromContext(ctx); len(attrs) != 1 || attrs[0].Value.String() != "r1" {
		t.Fatalf("AttrsFromContext() = %v; want request_id=r1", attrs)
	}
	if got := ContextWithAttrs(ctx); got != ctx {
		t.Fatal("ContextWithAttrs(ctx) returned a different context; want original")
	}
}

func TestAttrsFromContext_ReturnsCopy(t *testing.T) {
	input := []slog.Attr{slog.String("key", "original")}
	ctx := ContextWithAttrs(context.Background(), input...)
	input[0] = slog.String("key", "changed input")

	attrs := AttrsFromContext(ctx)
	attrs[0] = slog.String("key", "changed output")

	got := AttrsFromContext(ctx)
	if got[0].Value.String() != "original" {
		t.Fatalf("context attr = %q; want original", got[0].Value.String())
	}
}

func TestAttrsFromContext_NilOrEmptyReturnsNil(t *testing.T) {
	if got := AttrsFromContext(nil); got != nil { //nolint:staticcheck // 文档约定 nil ctx 返回 nil
		t.Fatalf("AttrsFromContext(nil) = %v; want nil", got)
	}
	if got := AttrsFromContext(context.Background()); got != nil {
		t.Fatalf("AttrsFromContext(background) = %v; want nil", got)
	}
}

func TestNewContextHandler_NilNextPanics(t *testing.T) {
	mustPanic(t, "newContextHandler()", func() { _ = newContextHandler(nil, attrsFromContext) })
}

func TestNewContextHandler_DisabledReturnsNext(t *testing.T) {
	next := &captureHandler{}
	if got := newContextHandler(next); got != next {
		t.Fatalf("newContextHandler() = %T; want next", got)
	}
}

func TestContextHandler_Handle(t *testing.T) {
	t.Run("single extractor clones record", func(t *testing.T) {
		next := &captureHandler{}
		handler := newContextHandler(next, func(context.Context) []slog.Attr {
			return []slog.Attr{slog.String("context_key", "context")}
		})
		record := slog.NewRecord(time.Now(), slog.LevelInfo, "handled", 0)
		record.AddAttrs(slog.String("call_key", "call"))
		if err := handler.Handle(context.Background(), record); err != nil {
			t.Fatal(err)
		}
		if record.NumAttrs() != 1 {
			t.Fatalf("original NumAttrs() = %d; want 1", record.NumAttrs())
		}
		if got := next.last().NumAttrs(); got != 2 {
			t.Fatalf("handled NumAttrs() = %d; want 2", got)
		}
	})
	t.Run("merges extractors without mutating returned attrs", func(t *testing.T) {
		first := make([]slog.Attr, 1, 2)
		first[0] = slog.String("request_id", "first")
		first[:cap(first)][1] = slog.String("sentinel", "unchanged")
		second := []slog.Attr{slog.String("request_id", "second")}
		third := []slog.Attr{slog.String("request_id", "third")}
		next := &captureHandler{}
		handler := newContextHandler(next,
			func(context.Context) []slog.Attr { return first },
			func(context.Context) []slog.Attr { return second },
			func(context.Context) []slog.Attr { return third },
		)
		if err := handler.Handle(context.Background(), slog.NewRecord(time.Now(), slog.LevelInfo, "handled", 0)); err != nil {
			t.Fatal(err)
		}
		if got := first[:cap(first)][1]; got.Key != "sentinel" || got.Value.String() != "unchanged" {
			t.Fatalf("first extractor spare capacity = %v; want sentinel=unchanged", got)
		}
		var values []string
		next.last().Attrs(func(attr slog.Attr) bool {
			values = append(values, attr.Value.String())
			return true
		})
		if len(values) != 3 || values[0] != "first" || values[1] != "second" || values[2] != "third" {
			t.Fatalf("merged values = %v; want [first second third]", values)
		}
	})
	t.Run("skips empty extractor results", func(t *testing.T) {
		next := &captureHandler{}
		handler := newContextHandler(next,
			func(context.Context) []slog.Attr { return nil },
			func(context.Context) []slog.Attr { return []slog.Attr{slog.String("request_id", "r1")} },
		)
		if err := handler.Handle(context.Background(), slog.NewRecord(time.Now(), slog.LevelInfo, "handled", 0)); err != nil {
			t.Fatal(err)
		}
		if got := attrString(next.last(), "request_id"); got != "r1" {
			t.Fatalf("request_id = %q; want r1", got)
		}
	})
}

func TestContextHandler_DuplicateKeys(t *testing.T) {
	t.Run("context and call", func(t *testing.T) {
		logger, output := newJSONLogger()
		ctx := ContextWithAttrs(context.Background(), slog.String("call_key", "context"))
		logger.InfoContext(ctx, "handled", slog.String("call_key", "call"))
		if count := strings.Count(output.String(), `"call_key":`); count != 2 {
			t.Fatalf("call_key count in %q = %d; want 2", output.String(), count)
		}
		got := mustJSONObject(t, output.Bytes())
		if got["call_key"] != "context" {
			t.Fatalf("unmarshaled call_key = %v; want last-win context", got["call_key"])
		}
	})
	t.Run("repeated ContextWithAttrs", func(t *testing.T) {
		logger, output := newJSONLogger()
		ctx := ContextWithAttrs(context.Background(), slog.String("request_id", "old"))
		ctx = ContextWithAttrs(ctx, slog.String("request_id", "new"))
		logger.InfoContext(ctx, "handled")
		if count := strings.Count(output.String(), `"request_id":`); count != 2 {
			t.Fatalf("request_id count in %q = %d; want 2", output.String(), count)
		}
		got := mustJSONObject(t, output.Bytes())
		if got["request_id"] != "new" {
			t.Fatalf("unmarshaled request_id = %v; want last-win new", got["request_id"])
		}
	})
	t.Run("multiple extractors", func(t *testing.T) {
		logger, output := newJSONLogger(
			WithContextExtractor(func(context.Context) []slog.Attr {
				return []slog.Attr{slog.String("source_key", "first")}
			}),
			WithContextExtractor(func(context.Context) []slog.Attr {
				return []slog.Attr{slog.String("source_key", "second")}
			}),
		)
		logger.InfoContext(context.Background(), "handled")
		if count := strings.Count(output.String(), `"source_key":`); count != 2 {
			t.Fatalf("source_key count in %q = %d; want 2", output.String(), count)
		}
		got := mustJSONObject(t, output.Bytes())
		if got["source_key"] != "second" {
			t.Fatalf("unmarshaled source_key = %v; want last-win second", got["source_key"])
		}
	})
}
