package log

import (
	"context"
	"log/slog"
)

// ContextExtractor 从一次日志调用的 context 中抽取 attrs。可能被并发调用，必须并发安全，且必须只读取 ctx。
type ContextExtractor func(context.Context) []slog.Attr

type contextAttrsKey struct{}

type contextHandler struct {
	next       slog.Handler
	extractors []ContextExtractor
}

// ContextWithAttrs 返回附带给定 attrs 的 ctx 副本。已有 attrs 保留，新 attrs 追加。
// 浅拷贝切片，值内引用不拷贝。ctx 为 nil 时改用 context.Background()，不保留取消或 deadline。
func ContextWithAttrs(ctx context.Context, attrs ...slog.Attr) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if len(attrs) == 0 {
		return ctx
	}
	prev := attrsFromContext(ctx)
	merged := make([]slog.Attr, 0, len(prev)+len(attrs))
	merged = append(merged, prev...)
	merged = append(merged, attrs...)
	return context.WithValue(ctx, contextAttrsKey{}, merged)
}

// AttrsFromContext 返回先前用 [ContextWithAttrs] 附上的 attrs 的浅拷贝。
// ctx 为 nil 或没有注入属性时返回 nil。
// 调用方可以修改返回的切片；属性值引用的 map、slice 或指针仍与 context 共享。
func AttrsFromContext(ctx context.Context) []slog.Attr {
	attrs := attrsFromContext(ctx)
	if len(attrs) == 0 {
		return nil
	}
	out := make([]slog.Attr, len(attrs))
	copy(out, attrs)
	return out
}

func attrsFromContext(ctx context.Context) []slog.Attr {
	if ctx == nil {
		return nil
	}
	attrs, _ := ctx.Value(contextAttrsKey{}).([]slog.Attr)
	return attrs
}

func newContextHandler(next slog.Handler, extractors ...ContextExtractor) slog.Handler {
	if next == nil {
		panic("nil log handler")
	}
	if len(extractors) == 0 {
		return next
	}
	return &contextHandler{next: next, extractors: extractors}
}

func (h *contextHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *contextHandler) Handle(ctx context.Context, record slog.Record) error {
	var attrs []slog.Attr
	copied := false
	for _, extractor := range h.extractors {
		nextAttrs := extractor(ctx)
		if len(nextAttrs) == 0 {
			continue
		}
		// 第一个直接赋值 不make
		if len(attrs) == 0 {
			attrs = nextAttrs
			continue
		}
		// 多个ctx attrs时，第一次 合并attrs和nextAttrs
		if !copied {
			//make 一块新数组，把已有内容拷进去再拼后面的。这样改的是自己的切片，碰不到 extractor 返回值的底层数组
			merged := make([]slog.Attr, 0, len(attrs)+len(nextAttrs))
			merged = append(merged, attrs...)
			attrs = append(merged, nextAttrs...)
			copied = true
		} else {
			// 后续直接append
			attrs = append(attrs, nextAttrs...)
		}
	}
	if len(attrs) == 0 {
		return h.next.Handle(ctx, record)
	}
	record = record.Clone()
	record.AddAttrs(attrs...)
	return h.next.Handle(ctx, record)
}

func (h *contextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	clone := *h
	clone.next = h.next.WithAttrs(attrs)
	return &clone
}

func (h *contextHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	clone := *h
	clone.next = h.next.WithGroup(name)
	return &clone
}
