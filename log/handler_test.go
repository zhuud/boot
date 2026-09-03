package log

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewHandler_NilWriterPanics(t *testing.T) {
	mustPanic(t, "NewHandler()", func() { _ = NewHandler(WithWriter(nil)) })
}

func TestNewHandler_IgnoresNilOption(t *testing.T) {
	logger, output := newJSONLogger(nil)
	logger.Info("ready")
	if !strings.Contains(output.String(), `"msg":"ready"`) {
		t.Fatalf("output = %q; want msg=ready", output.String())
	}
}

func TestNewHandler_Format(t *testing.T) {
	tests := []struct {
		name    string
		options []Option
		want    string
		wantNot string
	}{
		{name: "default text", want: "msg=ready", wantNot: `"msg"`},
		{name: "unknown falls back to text", options: []Option{WithFormat(Format(99))}, want: "msg=ready", wantNot: `"msg"`},
		{name: "json", options: []Option{WithFormat(FormatJSON)}, want: `"msg":"ready"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var output bytes.Buffer
			logger := slog.New(NewHandler(append([]Option{WithWriter(&output)}, tt.options...)...))
			logger.Info("ready")
			got := output.String()
			if !strings.Contains(got, tt.want) {
				t.Fatalf("output = %q; want contain %q", got, tt.want)
			}
			if tt.wantNot != "" && strings.Contains(got, tt.wantNot) {
				t.Fatalf("output = %q; want not contain %q", got, tt.wantNot)
			}
		})
	}
}

func TestNewHandler_Level(t *testing.T) {
	t.Run("filters debug at info", func(t *testing.T) {
		logger, output := newJSONLogger(WithLevel(slog.LevelInfo))
		logger.Debug("hidden")
		logger.Info("visible")
		got := output.String()
		if strings.Contains(got, "hidden") {
			t.Fatalf("output = %q; want debug filtered", got)
		}
		if !strings.Contains(got, `"msg":"visible"`) {
			t.Fatalf("output = %q; want msg=visible", got)
		}
	})
	t.Run("nil uses info default", func(t *testing.T) {
		logger, output := newJSONLogger(WithLevel(nil))
		logger.Debug("hidden")
		logger.Info("visible")
		got := output.String()
		if strings.Contains(got, "hidden") {
			t.Fatalf("output = %q; want debug filtered", got)
		}
		if !strings.Contains(got, `"msg":"visible"`) {
			t.Fatalf("output = %q; want msg=visible", got)
		}
	})
	t.Run("level var takes effect", func(t *testing.T) {
		var level slog.LevelVar
		level.Set(slog.LevelInfo)
		logger, output := newJSONLogger(WithLevel(&level))
		logger.Debug("hidden")
		level.Set(slog.LevelDebug)
		logger.Debug("visible")
		got := output.String()
		if strings.Contains(got, "hidden") {
			t.Fatalf("output = %q; want first debug filtered", got)
		}
		if !strings.Contains(got, `"msg":"visible"`) {
			t.Fatalf("output = %q; want msg=visible", got)
		}
	})
}

func TestNewHandler_Source(t *testing.T) {
	logger, output := newJSONLogger(WithSource(true))
	logger.Info("info")
	if _, ok := mustJSONObject(t, output.Bytes())[slog.SourceKey]; !ok {
		t.Fatal("source = absent; want present")
	}

	output.Reset()
	logger = slog.New(NewHandler(WithWriter(output), WithFormat(FormatJSON), WithSource(false)))
	logger.Info("info")
	if strings.Contains(output.String(), `"source"`) {
		t.Fatalf("output = %q; want no source", output.String())
	}
}

func TestNewHandler_LastScalarOptionWins(t *testing.T) {
	t.Run("format", func(t *testing.T) {
		var output bytes.Buffer
		logger := slog.New(NewHandler(
			WithWriter(&output),
			WithFormat(FormatJSON),
			WithFormat(FormatText),
		))
		logger.Info("ready")
		got := output.String()
		if !strings.Contains(got, "msg=ready") {
			t.Fatalf("output = %q; want text msg=ready", got)
		}
	})
	t.Run("source", func(t *testing.T) {
		logger, output := newJSONLogger(WithSource(true), WithSource(false))
		logger.Info("info")
		got := mustJSONObject(t, output.Bytes())
		if _, ok := got[slog.SourceKey]; ok {
			t.Fatalf("source = %v; want absent", got[slog.SourceKey])
		}
	})
	t.Run("attrGroup", func(t *testing.T) {
		logger, output := newJSONLogger(WithAttrGroup("arg"), WithAttrGroup(""))
		logger.Info("ready", slog.String("user_id", "u-1"))
		got := mustJSONObject(t, output.Bytes())
		if got["user_id"] != "u-1" {
			t.Fatalf("user_id = %v; want u-1 at root", got["user_id"])
		}
		if _, ok := got["arg"]; ok {
			t.Fatalf("arg = %v; want absent", got["arg"])
		}
	})
	t.Run("truncate", func(t *testing.T) {
		logger, output := newJSONLogger(WithTruncate(4), WithTruncate(0))
		logger.Info("abcdefghijk")
		got := output.String()
		if !strings.Contains(got, `"msg":"abcdefghijk"`) {
			t.Fatalf("output = %q; want full message", got)
		}
		if strings.Contains(got, truncatedKey) {
			t.Fatalf("output = %q; want no truncation", got)
		}
	})
	t.Run("sampling", func(t *testing.T) {
		logger, output := newJSONLogger(
			WithSampling(SamplingConfig{Interval: time.Hour, Initial: 1, Thereafter: 0}),
			WithSampling(SamplingConfig{}),
		)
		logger.Warn("storm")
		logger.Warn("storm")
		if got := strings.Count(output.String(), `"msg":"storm"`); got != 2 {
			t.Fatalf("storm count in %q = %d; want 2", output.String(), got)
		}
	})
	t.Run("dropFunc", func(t *testing.T) {
		logger, output := newJSONLogger(
			WithDropFunc(func(context.Context, slog.Record) bool { return true }),
			WithDropFunc(nil),
		)
		logger.Info("ready")
		if !strings.Contains(output.String(), `"msg":"ready"`) {
			t.Fatalf("output = %q; want msg=ready", output.String())
		}
	})
	t.Run("errorFunc", func(t *testing.T) {
		var calls atomic.Int32
		logger := slog.New(NewHandler(
			WithWriter(failWriter{}),
			WithErrorFunc(func(context.Context, slog.Record, error) { calls.Add(1) }),
			WithErrorFunc(nil),
		))
		logger.Info("ready")
		if got := calls.Load(); got != 0 {
			t.Fatalf("ErrorFunc calls = %d; want 0", got)
		}
	})
}

func TestNewHandler_RedactKeysAccumulate(t *testing.T) {
	logger, output := newJSONLogger(WithRedactKey("password"), WithRedactKey("token"))
	logger.Info("login", slog.String("password", "secret"), slog.String("token", "tok"))
	got := mustJSONObject(t, output.Bytes())
	if got["password"] != redactedValue {
		t.Fatalf("password = %v; want %q", got["password"], redactedValue)
	}
	if got["token"] != redactedValue {
		t.Fatalf("token = %v; want %q", got["token"], redactedValue)
	}
}

func TestNewHandler_ContextExtractor(t *testing.T) {
	t.Run("skips nil", func(t *testing.T) {
		logger, output := newJSONLogger(
			WithContextExtractor(nil, func(context.Context) []slog.Attr {
				return []slog.Attr{slog.String("trace_id", "t-1")}
			}),
		)
		logger.InfoContext(context.Background(), "handled")
		got := mustJSONObject(t, output.Bytes())
		if got["trace_id"] != "t-1" {
			t.Fatalf("trace_id = %v; want t-1", got["trace_id"])
		}
	})
	t.Run("default extracts ContextWithAttrs", func(t *testing.T) {
		logger, output := newJSONLogger()
		ctx := ContextWithAttrs(context.Background(), slog.String("request_id", "req-1"))
		logger.InfoContext(ctx, "handled")
		if !strings.Contains(output.String(), `"request_id":"req-1"`) {
			t.Fatalf("output = %q; want request_id", output.String())
		}
	})
}

func TestNewHandler_ErrorFunc(t *testing.T) {
	t.Run("reports write error", func(t *testing.T) {
		var calls atomic.Int32
		logger := slog.New(NewHandler(
			WithWriter(failWriter{}),
			WithFormat(FormatJSON),
			WithErrorFunc(func(_ context.Context, _ slog.Record, err error) {
				if err == nil {
					t.Error("ErrorFunc err = nil; want write error")
				}
				calls.Add(1)
			}),
		))
		logger.Info("ready")
		if got := calls.Load(); got != 1 {
			t.Fatalf("ErrorFunc calls = %d; want 1", got)
		}
	})
	t.Run("no default callback on write error", func(_ *testing.T) {
		logger := slog.New(NewHandler(WithWriter(failWriter{}), WithFormat(FormatJSON)))
		logger.Info("ready")
	})
	t.Run("recovers base panic without callback", func(_ *testing.T) {
		logger := slog.New(NewHandler(WithWriter(panicWriter{})))
		logger.Info("ready")
	})
	t.Run("does not catch decorator panic", func(t *testing.T) {
		var calls atomic.Int32
		mustPanic(t, "Handle()", func() {
			logger := slog.New(NewHandler(
				WithWriter(&bytes.Buffer{}),
				WithDropFunc(func(context.Context, slog.Record) bool { panic("drop") }),
				WithErrorFunc(func(context.Context, slog.Record, error) { calls.Add(1) }),
			))
			logger.Info("ready")
		})
		if got := calls.Load(); got != 0 {
			t.Fatalf("ErrorFunc calls = %d; want 0", got)
		}
	})
}

func TestNewHandler_ConcurrentHandle(t *testing.T) {
	var output syncBuffer
	logger := slog.New(NewHandler(
		WithWriter(&output),
		WithFormat(FormatJSON),
		WithAttrGroup("arg"),
		WithRedactKey("password"),
		WithTruncate(2048),
		WithSampling(SamplingConfig{Interval: time.Hour, Initial: 1 << 20, Thereafter: 1}),
	))
	const workers = 32
	const perWorker = 32
	start := make(chan struct{})
	var wait sync.WaitGroup
	wait.Add(workers)
	for range workers {
		go func() {
			defer wait.Done()
			<-start
			ctx := ContextWithAttrs(context.Background(), slog.String("request_id", "r1"))
			for range perWorker {
				logger.InfoContext(ctx, "ready", slog.String("user_id", "u-1"), slog.String("password", "secret"))
			}
		}()
	}
	close(start)
	wait.Wait()

	lines := jsonLines(t, output.bytes())
	if got := len(lines); got != workers*perWorker {
		t.Fatalf("line count = %d; want %d", got, workers*perWorker)
	}
	for i, line := range lines {
		if line["request_id"] != "r1" {
			t.Fatalf("lines[%d].request_id = %v; want r1", i, line["request_id"])
		}
		arg, ok := line["arg"].(map[string]any)
		if !ok || arg["user_id"] != "u-1" || arg["password"] != redactedValue {
			t.Fatalf("lines[%d].arg = %v; want user_id=u-1 password=%q", i, line["arg"], redactedValue)
		}
	}
}
