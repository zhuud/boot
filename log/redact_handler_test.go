package log

import (
	"context"
	"log/slog"
	"testing"
	"time"
)

func TestNewRedactHandler_NilNextPanics(t *testing.T) {
	mustPanic(t, "newRedactHandler()", func() { _ = newRedactHandler(nil, "password") })
}

func TestNewRedactHandler_DisabledReturnsNext(t *testing.T) {
	next := &captureHandler{}
	if got := newRedactHandler(next); got != next {
		t.Fatalf("empty keys = %T; want next", got)
	}
	if got := newRedactHandler(next, "", ""); got != next {
		t.Fatalf("blank keys = %T; want next", got)
	}
}

func TestNewRedactHandler_SkipsEmptyKeys(t *testing.T) {
	next := &captureHandler{}
	handler := newRedactHandler(next, "", "password")
	if handler == next {
		t.Fatal("newRedactHandler() = next; want wrapped handler")
	}
	record := slog.NewRecord(time.Now(), slog.LevelInfo, "login", 0)
	record.AddAttrs(slog.String("password", "secret"), slog.String("id", "1"))
	if err := handler.Handle(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	if got := attrString(next.last(), "password"); got != redactedValue {
		t.Fatalf("password = %q; want %q", got, redactedValue)
	}
	if got := attrString(next.last(), "id"); got != "1" {
		t.Fatalf("id = %q; want 1", got)
	}
}

func TestRedactHandler_Handle(t *testing.T) {
	t.Run("empty record", func(t *testing.T) {
		next := &captureHandler{}
		handler := newRedactHandler(next, "password")
		record := slog.NewRecord(time.Now(), slog.LevelInfo, "ready", 0)
		if err := handler.Handle(context.Background(), record); err != nil {
			t.Fatal(err)
		}
		got := next.last()
		if got.Message != "ready" || got.NumAttrs() != 0 {
			t.Fatalf("record = %q with %d attrs; want ready with 0 attrs", got.Message, got.NumAttrs())
		}
	})
	t.Run("unchanged attrs pass through", func(t *testing.T) {
		next := &captureHandler{}
		handler := newRedactHandler(next, "password")
		record := slog.NewRecord(time.Now(), slog.LevelInfo, "ok", 0)
		record.AddAttrs(slog.String("id", "1"), slog.Int("n", 2))
		if err := handler.Handle(context.Background(), record); err != nil {
			t.Fatal(err)
		}
		got := next.last()
		if got.NumAttrs() != 2 {
			t.Fatalf("NumAttrs() = %d; want 2", got.NumAttrs())
		}
		if attrString(got, "id") != "1" {
			t.Fatalf("id = %q; want 1", attrString(got, "id"))
		}
		if attrString(got, "n") != "2" {
			t.Fatalf("n = %q; want 2", attrString(got, "n"))
		}

		passthrough := newRedactHandler(slog.DiscardHandler, "password")
		allocs := testing.AllocsPerRun(100, func() {
			_ = passthrough.Handle(context.Background(), record)
		})
		if allocs != 0 {
			t.Fatalf("unchanged Handle allocs = %v; want 0", allocs)
		}
	})
	t.Run("group path and leaf key", func(t *testing.T) {
		next := &captureHandler{}
		handler := newRedactHandler(next, "user.token", "password")
		record := slog.NewRecord(time.Now(), slog.LevelInfo, "login", 0)
		record.AddAttrs(slog.Group("user",
			slog.String("token", "tok"),
			slog.String("password", "secret"),
			slog.String("id", "1"),
		))
		if err := handler.Handle(context.Background(), record); err != nil {
			t.Fatal(err)
		}
		group := attrGroup(next.last(), "user")
		if group["token"] != redactedValue {
			t.Fatalf("user.token = %q; want %q", group["token"], redactedValue)
		}
		if group["password"] != redactedValue {
			t.Fatalf("user.password = %q; want %q", group["password"], redactedValue)
		}
		if group["id"] != "1" {
			t.Fatalf("user.id = %q; want 1", group["id"])
		}
	})
	t.Run("path does not match top-level key", func(t *testing.T) {
		next := &captureHandler{}
		handler := newRedactHandler(next, "user.token")
		record := slog.NewRecord(time.Now(), slog.LevelInfo, "login", 0)
		record.AddAttrs(slog.String("token", "tok"))
		if err := handler.Handle(context.Background(), record); err != nil {
			t.Fatal(err)
		}
		if got := attrString(next.last(), "token"); got != "tok" {
			t.Fatalf("token = %q; want tok", got)
		}
	})
	t.Run("anonymous group still matches leaf key", func(t *testing.T) {
		next := &captureHandler{}
		handler := newRedactHandler(next, "password")
		record := slog.NewRecord(time.Now(), slog.LevelInfo, "login", 0)
		record.AddAttrs(slog.Group("", slog.String("password", "secret"), slog.String("id", "1")))
		if err := handler.Handle(context.Background(), record); err != nil {
			t.Fatal(err)
		}
		group := attrGroup(next.last(), "")
		if group["password"] != redactedValue {
			t.Fatalf("password = %q; want %q", group["password"], redactedValue)
		}
		if group["id"] != "1" {
			t.Fatalf("id = %q; want 1", group["id"])
		}
	})
	t.Run("leaf key matches inside named group", func(t *testing.T) {
		next := &captureHandler{}
		handler := newRedactHandler(next, "password")
		record := slog.NewRecord(time.Now(), slog.LevelInfo, "login", 0)
		record.AddAttrs(slog.Group("user", slog.String("password", "secret"), slog.String("id", "1")))
		if err := handler.Handle(context.Background(), record); err != nil {
			t.Fatal(err)
		}
		group := attrGroup(next.last(), "user")
		if group["password"] != redactedValue {
			t.Fatalf("user.password = %q; want %q", group["password"], redactedValue)
		}
		if group["id"] != "1" {
			t.Fatalf("user.id = %q; want 1", group["id"])
		}
	})
	t.Run("group preserves resolved LogValuer", func(t *testing.T) {
		next := &captureHandler{}
		handler := newRedactHandler(next, "password")
		calls := 0
		record := slog.NewRecord(time.Now(), slog.LevelInfo, "ok", 0)
		record.AddAttrs(slog.Group("request",
			slog.Any("method", countingLogValuer{calls: &calls, value: slog.StringValue("GET")}),
		))
		if err := handler.Handle(context.Background(), record); err != nil {
			t.Fatal(err)
		}
		if group := attrGroup(next.last(), "request"); group["method"] != "GET" {
			t.Fatalf("request.method = %q; want GET", group["method"])
		}
		if calls != 1 {
			t.Fatalf("LogValue calls = %d; want 1", calls)
		}
	})
}

func TestRedactHandler_WithGroupUsesPath(t *testing.T) {
	next := &captureHandler{}
	handler := newRedactHandler(next, "request.password").WithGroup("request")
	record := slog.NewRecord(time.Now(), slog.LevelInfo, "login", 0)
	record.AddAttrs(slog.String("password", "secret"))
	if err := handler.Handle(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	if got := attrString(next.last(), "password"); got != redactedValue {
		t.Fatalf("password = %q; want %q", got, redactedValue)
	}
}

func TestRedactHandler_WithAttrsRedacts(t *testing.T) {
	logger, output := newJSONLogger(WithRedactKey("password"))
	logger.With(slog.String("password", "secret"), slog.String("id", "1")).Info("login")
	got := mustJSONObject(t, output.Bytes())
	if got["password"] != redactedValue {
		t.Fatalf("password = %v; want %q", got["password"], redactedValue)
	}
	if got["id"] != "1" {
		t.Fatalf("id = %v; want 1", got["id"])
	}
}

func TestRedactAttr_UnchangedGroupDoesNotReportChange(t *testing.T) {
	handler := &redactHandler{keys: map[string]struct{}{"password": {}}}
	attr := slog.Group("request", slog.String("method", "GET"))
	got, changed := handler.redactAttr(nil, attr)
	if changed {
		t.Fatal("redactAttr() changed = true; want false")
	}
	if group := got.Value.Group(); len(group) != 1 || group[0].Value.String() != "GET" {
		t.Fatalf("redactAttr() group = %v; want method=GET", group)
	}
	allocs := testing.AllocsPerRun(100, func() {
		_, _ = handler.redactAttr(nil, attr)
	})
	if allocs != 0 {
		t.Fatalf("unchanged group allocs = %v; want 0", allocs)
	}
}
