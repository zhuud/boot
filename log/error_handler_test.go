package log

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewErrorHandler_NilNextPanics(t *testing.T) {
	mustPanic(t, "newErrorHandler()", func() {
		_ = newErrorHandler(nil, func(context.Context, slog.Record, error) {})
	})
}

func TestNewErrorHandler_NilStillWraps(t *testing.T) {
	next := fixedErrorHandler{err: errors.New("write failed")}
	got := newErrorHandler(next, nil)
	if _, ok := got.(*errorHandler); !ok {
		t.Fatalf("type = %T; want *errorHandler", got)
	}
	record := slog.NewRecord(time.Now(), slog.LevelError, "failed", 0)
	if err := got.Handle(context.Background(), record); !errors.Is(err, next.err) {
		t.Fatalf("Handle() error = %v; want %v", err, next.err)
	}
}

func TestErrorHandler_Handle(t *testing.T) {
	t.Run("does not call on success", func(t *testing.T) {
		next := &captureHandler{}
		calls := 0
		handler := newErrorHandler(next, func(context.Context, slog.Record, error) {
			calls++
		})
		record := slog.NewRecord(time.Now(), slog.LevelInfo, "ready", 0)
		if err := handler.Handle(context.Background(), record); err != nil {
			t.Fatalf("Handle() error = %v; want nil", err)
		}
		if calls != 0 {
			t.Fatalf("ErrorFunc calls = %d; want 0", calls)
		}
		if got := next.last().Message; got != "ready" {
			t.Fatalf("message = %q; want ready", got)
		}
	})
	t.Run("reports returned error", func(t *testing.T) {
		want := errors.New("write failed")
		var reported error
		handler := newErrorHandler(fixedErrorHandler{err: want}, func(_ context.Context, _ slog.Record, err error) {
			reported = err
		})
		got := handler.Handle(context.Background(), slog.NewRecord(time.Now(), slog.LevelError, "failed", 0))
		if !errors.Is(got, want) {
			t.Fatalf("Handle() error = %v; want %v", got, want)
		}
		if !errors.Is(reported, want) {
			t.Fatalf("ErrorFunc error = %v; want %v", reported, want)
		}
	})
	t.Run("recovers panic without callback", func(t *testing.T) {
		handler := newErrorHandler(panicHandler{}, nil)
		err := handler.Handle(context.Background(), slog.NewRecord(time.Now(), slog.LevelInfo, "x", 0))
		if err == nil {
			t.Fatal("Handle() error = nil; want recovered panic")
		}
		if !strings.Contains(err.Error(), "log downstream handler panic: boom") {
			t.Fatalf("Handle() error = %v; want recovered panic", err)
		}
	})
	t.Run("recovers panic with callback and stack", func(t *testing.T) {
		var reported error
		handler := newErrorHandler(panicHandler{}, func(_ context.Context, _ slog.Record, err error) {
			reported = err
		})
		err := handler.Handle(context.Background(), slog.NewRecord(time.Now(), slog.LevelInfo, "x", 0))
		if err == nil {
			t.Fatal("Handle() error = nil; want panic error")
		}
		if reported == nil || !strings.Contains(reported.Error(), "log downstream handler panic: boom") {
			t.Fatalf("ErrorFunc error = %v; want recovered panic", reported)
		}
		if !strings.Contains(reported.Error(), "goroutine") {
			t.Fatalf("ErrorFunc error = %v; want stack", reported)
		}
	})
}

func TestErrorHandler_ConcurrentCalls(t *testing.T) {
	const workers = 64
	var calls atomic.Int32
	handler := newErrorHandler(fixedErrorHandler{err: errors.New("write failed")}, func(context.Context, slog.Record, error) {
		calls.Add(1)
	})
	start := make(chan struct{})
	var wait sync.WaitGroup
	wait.Add(workers)
	for range workers {
		go func() {
			defer wait.Done()
			<-start
			_ = handler.Handle(context.Background(), slog.NewRecord(time.Now(), slog.LevelInfo, "ready", 0))
		}()
	}
	close(start)
	wait.Wait()
	if got := calls.Load(); got != workers {
		t.Fatalf("ErrorFunc calls = %d; want %d", got, workers)
	}
}

type fixedErrorHandler struct {
	err error
}

func (h fixedErrorHandler) Enabled(context.Context, slog.Level) bool { return true }
func (h fixedErrorHandler) Handle(context.Context, slog.Record) error {
	return h.err
}
func (h fixedErrorHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h fixedErrorHandler) WithGroup(string) slog.Handler      { return h }

type panicHandler struct{}

func (panicHandler) Enabled(context.Context, slog.Level) bool { return true }
func (panicHandler) Handle(context.Context, slog.Record) error {
	panic("boom")
}
func (panicHandler) WithAttrs([]slog.Attr) slog.Handler { return panicHandler{} }
func (panicHandler) WithGroup(string) slog.Handler      { return panicHandler{} }
