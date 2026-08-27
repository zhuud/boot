package log

import (
	"context"
	"log/slog"
	"unicode/utf8"
)

const truncatedKey = "log.truncated"

type truncateHandler struct {
	next      slog.Handler
	maxBytes  int
	truncated bool
}

// 按 maxBytes 截断记录 message 以及 string / []byte 属性值。字符串截断对齐 UTF-8 rune 边界。
// 发生截断时写入 slog.Bool("log.truncated", true)。
func newTruncateHandler(next slog.Handler, maxBytes int) slog.Handler {
	if next == nil {
		panic("nil log handler")
	}
	if maxBytes <= 0 {
		return next
	}
	return &truncateHandler{next: next, maxBytes: maxBytes}
}

func (h *truncateHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *truncateHandler) Handle(ctx context.Context, record slog.Record) error {
	message, messageTruncated := truncateUTF8(record.Message, h.maxBytes)
	if !messageTruncated && record.NumAttrs() == 0 {
		return h.next.Handle(ctx, record)
	}

	var out slog.Record
	// changed 是否换新 record（含 Resolve LogValuer）；truncated 是否真截断，用来写 log.truncated。
	truncated := messageTruncated
	changed := messageTruncated
	if messageTruncated {
		out = slog.NewRecord(record.Time, record.Level, message, record.PC)
	}
	prefixLen := 0
	record.Attrs(func(attr slog.Attr) bool {
		truncatedAttr, attrChanged, attrTruncated := h.truncateAttr(attr)
		truncated = truncated || attrTruncated
		if !changed {
			if !attrChanged {
				prefixLen++
				return true
			}
			out = slog.NewRecord(record.Time, record.Level, message, record.PC)
			copyRecordPrefix(&out, record, prefixLen)
			changed = true
		}
		out.AddAttrs(truncatedAttr)
		prefixLen++
		return true
	})
	if !changed {
		return h.next.Handle(ctx, record)
	}
	if truncated && !h.truncated {
		out.AddAttrs(slog.Bool(truncatedKey, true))
	}
	return h.next.Handle(ctx, out)
}

func (h *truncateHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	out := make([]slog.Attr, 0, len(attrs)+1)
	truncated := h.truncated
	for _, attr := range attrs {
		truncatedAttr, _, attrTruncated := h.truncateAttr(attr)
		out = append(out, truncatedAttr)
		truncated = truncated || attrTruncated
	}
	if truncated && !h.truncated {
		out = append(out, slog.Bool(truncatedKey, true))
	}

	clone := *h
	clone.next = h.next.WithAttrs(out)
	clone.truncated = truncated
	return &clone
}

func (h *truncateHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	clone := *h
	clone.next = h.next.WithGroup(name)
	return &clone
}

// 未改写时返回原 attr，避免 group 热路径分配。后两个返回值分别是是否变化（含 Resolve LogValuer）与是否截断。
func (h *truncateHandler) truncateAttr(attr slog.Attr) (slog.Attr, bool, bool) {
	// Resolve 之后 Kind 会变  即使没截断，只要展开过 LogValuer 也必须拷贝，不能复用原切片
	changed := attr.Value.Kind() == slog.KindLogValuer

	attr.Value = attr.Value.Resolve()
	switch attr.Value.Kind() {
	case slog.KindString:
		value, truncated := truncateUTF8(attr.Value.String(), h.maxBytes)
		if !truncated {
			return attr, changed, false
		}
		return slog.String(attr.Key, value), true, true

	case slog.KindAny:
		data, ok := attr.Value.Any().([]byte)
		if !ok || len(data) <= h.maxBytes {
			return attr, changed, false
		}
		truncated := make([]byte, h.maxBytes)
		copy(truncated, data[:h.maxBytes])
		return slog.Any(attr.Key, truncated), true, true

	case slog.KindGroup:
		group := attr.Value.Group()
		truncated := false
		// group 无需改写时不分配；出现第一个变化才拷贝。不能原地改 slog 内部切片。
		for i, groupAttr := range group {
			truncatedAttr, attrChanged, attrTruncated := h.truncateAttr(groupAttr)
			truncated = truncated || attrTruncated
			if !attrChanged {
				continue
			}
			// COW
			truncatedGroup := make([]slog.Attr, len(group))
			// 取前面循环已经处理未截断数据
			copy(truncatedGroup, group[:i])
			// 当前结果
			truncatedGroup[i] = truncatedAttr
			// 继续循环后面看是否需要截断并且赋值
			for j := i + 1; j < len(group); j++ {
				var nextTruncated bool
				truncatedGroup[j], _, nextTruncated = h.truncateAttr(group[j])
				truncated = truncated || nextTruncated
			}
			return slog.Attr{Key: attr.Key, Value: slog.GroupValue(truncatedGroup...)}, true, truncated
		}
		return attr, changed, false

	default:
		return attr, changed, false
	}
}

// truncateUTF8 按字节上限截断，并回退到 UTF-8 rune 边界，避免切出非法序列。
func truncateUTF8(value string, maxBytes int) (string, bool) {
	if len(value) <= maxBytes {
		return value, false
	}
	end := maxBytes
	for end > 0 && !utf8.RuneStart(value[end]) {
		end--
	}
	return value[:end], true
}
