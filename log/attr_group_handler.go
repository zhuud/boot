package log

import (
	"context"
	"log/slog"
)

type attrGroupHandler struct {
	next  slog.Handler
	group string
}

// 空 group 返回 next。
func newAttrGroupHandler(next slog.Handler, group string) slog.Handler {
	if next == nil {
		panic("nil log handler")
	}
	if group == "" {
		return next
	}
	return &attrGroupHandler{next: next, group: group}
}

func (h *attrGroupHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *attrGroupHandler) Handle(ctx context.Context, record slog.Record) error {
	if record.NumAttrs() == 0 {
		return h.next.Handle(ctx, record)
	}
	attrs := make([]slog.Attr, 0, record.NumAttrs())
	record.Attrs(func(attr slog.Attr) bool {
		attrs = append(attrs, attr)
		return true
	})

	grouped := slog.NewRecord(record.Time, record.Level, record.Message, record.PC)
	grouped.AddAttrs(slog.Attr{Key: h.group, Value: slog.GroupValue(attrs...)})
	return h.next.Handle(ctx, grouped)
}

func (h *attrGroupHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	clone := *h
	clone.next = h.next.WithAttrs(attrs)
	return &clone
}

func (h *attrGroupHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	clone := *h
	clone.next = h.next.WithGroup(name)
	return &clone
}
