package log

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
)

// ErrorFunc 观测基座写出失败或被恢复的 panic。可能被并发调用，必须并发安全，且不得经同一 Logger 写回。
type ErrorFunc func(context.Context, slog.Record, error)

type errorHandler struct {
	next      slog.Handler
	errorFunc ErrorFunc
}

// 只恢复基座 panic，转为带堆栈的 error，自身 panic 不恢复。
// 回调收到的 record 已经过外层装饰。
func newErrorHandler(next slog.Handler, errorFunc ErrorFunc) slog.Handler {
	if next == nil {
		panic("nil log handler")
	}

	return &errorHandler{next: next, errorFunc: errorFunc}
}

func (h *errorHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *errorHandler) Handle(ctx context.Context, record slog.Record) error {
	err := func() (err error) {
		defer func() {
			if recovered := recover(); recovered != nil {
				err = fmt.Errorf("log downstream handler panic: %v\n%s", recovered, debug.Stack())
			}
		}()
		return h.next.Handle(ctx, record)
	}()
	if err != nil && h.errorFunc != nil {
		h.errorFunc(ctx, record, err)
	}
	return err
}

func (h *errorHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	clone := *h
	clone.next = h.next.WithAttrs(attrs)
	return &clone
}

func (h *errorHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	clone := *h
	clone.next = h.next.WithGroup(name)
	return &clone
}
