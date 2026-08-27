package log

import (
	"bytes"
	"context"
	"log/slog"
	"testing"
	"time"
)

func TestNewAttrGroupHandler_NilNextPanics(t *testing.T) {
	mustPanic(t, "newAttrGroupHandler()", func() { _ = newAttrGroupHandler(nil, "arg") })
}

func TestNewAttrGroupHandler_DisabledReturnsNext(t *testing.T) {
	next := &captureHandler{}
	if got := newAttrGroupHandler(next, ""); got != next {
		t.Fatalf("newAttrGroupHandler() = %T; want next", got)
	}
}

func TestAttrGroupHandler_Handle(t *testing.T) {
	t.Run("wraps call attrs", func(t *testing.T) {
		next := &captureHandler{}
		handler := newAttrGroupHandler(next, "arg")
		at := time.Unix(1_700_000_000, 0)
		record := slog.NewRecord(at, slog.LevelWarn, "user created", 42)
		record.AddAttrs(slog.String("user_id", "u-1"))
		if err := handler.Handle(context.Background(), record); err != nil {
			t.Fatal(err)
		}

		got := next.last()
		if got.Time != at || got.Level != slog.LevelWarn || got.Message != "user created" || got.PC != 42 {
			t.Fatalf("record metadata = %v, %v, %q, %d; want original metadata", got.Time, got.Level, got.Message, got.PC)
		}
		if got.NumAttrs() != 1 {
			t.Fatalf("NumAttrs() = %d; want 1", got.NumAttrs())
		}
		if attrString(got, "user_id") != "" {
			t.Fatalf("root user_id = %q; want empty", attrString(got, "user_id"))
		}
		if group := attrGroup(got, "arg"); group["user_id"] != "u-1" {
			t.Fatalf("arg.user_id = %q; want u-1", group["user_id"])
		}
	})
	t.Run("skips empty record", func(t *testing.T) {
		next := &captureHandler{}
		handler := newAttrGroupHandler(next, "arg")
		record := slog.NewRecord(time.Now(), slog.LevelInfo, "ready", 0)
		if err := handler.Handle(context.Background(), record); err != nil {
			t.Fatal(err)
		}
		if got := next.last().NumAttrs(); got != 0 {
			t.Fatalf("NumAttrs() = %d; want 0", got)
		}
	})
}

func TestAttrGroupHandler_WithAttrsStayAtRoot(t *testing.T) {
	var output bytes.Buffer
	handler := newAttrGroupHandler(slog.NewJSONHandler(&output, nil), "arg")
	handler = handler.WithAttrs([]slog.Attr{slog.String("service", "api")})
	record := slog.NewRecord(time.Now(), slog.LevelInfo, "ready", 0)
	record.AddAttrs(slog.String("user_id", "u-1"))
	if err := handler.Handle(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	got := mustJSONObject(t, output.Bytes())
	if got["service"] != "api" {
		t.Fatalf("service = %v; want api", got["service"])
	}
	if _, ok := got["user_id"]; ok {
		t.Fatalf("user_id at root = %v; want absent", got["user_id"])
	}
	arg, ok := got["arg"].(map[string]any)
	if !ok || arg["user_id"] != "u-1" {
		t.Fatalf("arg = %v; want user_id=u-1", got["arg"])
	}
}

func TestAttrGroupHandler_WithGroupNestsCallAttrs(t *testing.T) {
	var output bytes.Buffer
	handler := newAttrGroupHandler(slog.NewJSONHandler(&output, nil), "arg")
	handler = handler.WithGroup("request")
	record := slog.NewRecord(time.Now(), slog.LevelInfo, "ready", 0)
	record.AddAttrs(slog.String("user_id", "u-1"))
	if err := handler.Handle(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	got := mustJSONObject(t, output.Bytes())
	request, ok := got["request"].(map[string]any)
	if !ok {
		t.Fatalf("request = %v; want object", got["request"])
	}
	arg, ok := request["arg"].(map[string]any)
	if !ok || arg["user_id"] != "u-1" {
		t.Fatalf("request.arg = %v; want user_id=u-1", request["arg"])
	}
}
