package log

import (
	"context"
	"log/slog"
	"strings"
)

const redactedValue = "***"

type redactHandler struct {
	next   slog.Handler
	keys   map[string]struct{}
	groups []string
}

// 对匹配叶子名或点分组路径的属性值脱敏为 "***"。
// 空字符串 key 会被忽略；忽略后没有任何 key 时返回 next。
func newRedactHandler(next slog.Handler, keys ...string) slog.Handler {
	if next == nil {
		panic("nil log handler")
	}
	if len(keys) == 0 {
		return next
	}
	uniqueKeys := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		if key == "" {
			continue
		}
		uniqueKeys[key] = struct{}{}
	}
	if len(uniqueKeys) == 0 {
		return next
	}
	return &redactHandler{next: next, keys: uniqueKeys}
}

func (h *redactHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *redactHandler) Handle(ctx context.Context, record slog.Record) error {
	if record.NumAttrs() == 0 {
		return h.next.Handle(ctx, record)
	}

	var out slog.Record
	changed := false
	prefixLen := 0
	record.Attrs(func(attr slog.Attr) bool {
		redactedAttr, attrChanged := h.redactAttr(h.groups, attr)
		if !changed {
			if !attrChanged {
				prefixLen++
				return true
			}
			out = slog.NewRecord(record.Time, record.Level, record.Message, record.PC)
			copyRecordPrefix(&out, record, prefixLen)
			changed = true
		}
		out.AddAttrs(redactedAttr)
		prefixLen++
		return true
	})
	if !changed {
		return h.next.Handle(ctx, record)
	}
	return h.next.Handle(ctx, out)
}

func (h *redactHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	out := make([]slog.Attr, len(attrs))
	for i, attr := range attrs {
		out[i], _ = h.redactAttr(h.groups, attr)
	}

	clone := *h
	clone.next = h.next.WithAttrs(out)
	return &clone
}

func (h *redactHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	clone := *h
	clone.groups = appendGroupPath(h.groups, name)
	clone.next = h.next.WithGroup(name)
	return &clone
}

// 未改写时返回原 attr，避免 group 热路径分配。bool 表示相对入参是否变化（含 Resolve LogValuer）。
func (h *redactHandler) redactAttr(groups []string, attr slog.Attr) (slog.Attr, bool) {
	changed := attr.Value.Kind() == slog.KindLogValuer

	attr.Value = attr.Value.Resolve()
	if attr.Value.Kind() == slog.KindGroup {
		group := attr.Value.Group()
		groupPath := appendGroupPath(groups, attr.Key)
		// group 无变化时不分配；出现第一个变化才拷贝。不能原地改 slog 内部切片。
		for i, groupAttr := range group {
			redactedAttr, attrChanged := h.redactAttr(groupPath, groupAttr)
			if !attrChanged {
				continue
			}
			// COW
			redactedGroup := make([]slog.Attr, len(group))
			// 取前面循环已经处理未脱敏数据
			copy(redactedGroup, group[:i])
			// 当前结果
			redactedGroup[i] = redactedAttr
			// 继续循环后面看是否需要脱敏并且赋值
			for j := i + 1; j < len(group); j++ {
				redactedGroup[j], _ = h.redactAttr(groupPath, group[j])
			}
			return slog.Attr{Key: attr.Key, Value: slog.GroupValue(redactedGroup...)}, true
		}
		return attr, changed
	}

	if h.shouldRedact(groups, attr.Key) {
		return slog.Attr{Key: attr.Key, Value: slog.StringValue(redactedValue)}, true
	}
	return attr, changed
}

func (h *redactHandler) shouldRedact(groups []string, key string) bool {
	if _, ok := h.keys[key]; ok {
		return true
	}
	if len(groups) == 0 {
		return false
	}
	path := strings.Join(appendGroupPath(groups, key), ".")
	_, ok := h.keys[path]
	return ok
}

// appendGroupPath 复制 groups 再追加 key，避免派生 handler 改写共享底层数组。
// key 为空时原样返回，不分配。
func appendGroupPath(groups []string, key string) []string {
	if key == "" {
		return groups
	}
	path := make([]string, 0, len(groups)+1)
	path = append(path, groups...)
	path = append(path, key)
	return path
}

// copyRecordPrefix 把 src 的前 prefixLen 个 attrs 追加到 dst。prefixLen<=0 时不访问 src。
func copyRecordPrefix(dst *slog.Record, src slog.Record, prefixLen int) {
	if prefixLen <= 0 {
		return
	}
	copied := 0
	src.Attrs(func(attr slog.Attr) bool {
		if copied >= prefixLen {
			return false
		}
		dst.AddAttrs(attr)
		copied++
		return true
	})
}
