package log

import (
	"context"
	"log/slog"
)

// DropFunc 判断是否丢弃一条记录。返回 true 时丢弃。可能被并发调用，必须并发安全且不可阻塞。
type DropFunc func(context.Context, slog.Record) bool

type dropHandler struct {
	next     slog.Handler
	dropFunc DropFunc
}

// 在 Context attrs 与脱敏之后运行，能看到本次调用与 Context 抽出的 attrs，不含 Logger.With 预绑定属性。
func newDropHandler(next slog.Handler, dropFunc DropFunc) slog.Handler {
	if next == nil {
		panic("nil log handler")
	}
	if dropFunc == nil {
		return next
	}
	return &dropHandler{next: next, dropFunc: dropFunc}
}

func (h *dropHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *dropHandler) Handle(ctx context.Context, record slog.Record) error {
	if h.dropFunc(ctx, record) {
		return nil
	}
	return h.next.Handle(ctx, record)
}

func (h *dropHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	clone := *h
	clone.next = h.next.WithAttrs(attrs)
	return &clone
}

func (h *dropHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	clone := *h
	clone.next = h.next.WithGroup(name)
	return &clone
}
