package log

import (
	"context"
	"log/slog"
	"testing"
	"time"
)

func TestNewDropHandler_NilNextPanics(t *testing.T) {
	mustPanic(t, "newDropHandler()", func() {
		_ = newDropHandler(nil, func(context.Context, slog.Record) bool { return false })
	})
}

func TestNewDropHandler_DisabledReturnsNext(t *testing.T) {
	next := &captureHandler{}
	if got := newDropHandler(next, nil); got != next {
		t.Fatalf("newDropHandler() = %T; want next", got)
	}
}

func TestDropHandler_Handle(t *testing.T) {
	next := &captureHandler{}
	calls := 0
	handler := newDropHandler(next, func(_ context.Context, record slog.Record) bool {
		calls++
		return record.Message == "health"
	})
	now := time.Now()
	for _, message := range []string{"health", "ready"} {
		if err := handler.Handle(context.Background(), slog.NewRecord(now, slog.LevelInfo, message, 0)); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 2 {
		t.Fatalf("DropFunc calls = %d; want 2", calls)
	}
	if len(next.records) != 1 || next.last().Message != "ready" {
		t.Fatalf("passed records = %v; want only ready", next.records)
	}
}
