package log

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestNewTruncateHandler_NilNextPanics(t *testing.T) {
	mustPanic(t, "newTruncateHandler()", func() { _ = newTruncateHandler(nil, 4) })
}

func TestNewTruncateHandler_DisabledReturnsNext(t *testing.T) {
	next := &captureHandler{}
	for _, maxBytes := range []int{0, -1} {
		if got := newTruncateHandler(next, maxBytes); got != next {
			t.Fatalf("maxBytes=%d = %T; want next", maxBytes, got)
		}
	}
}

func TestTruncateHandler_Handle(t *testing.T) {
	t.Run("unchanged empty record", func(t *testing.T) {
		next := &captureHandler{}
		handler := newTruncateHandler(next, 8)
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
		handler := newTruncateHandler(next, 32)
		record := slog.NewRecord(time.Now(), slog.LevelInfo, "ready", 0)
		record.AddAttrs(slog.String("id", "1"), slog.Group("request", slog.String("method", "GET")))
		if err := handler.Handle(context.Background(), record); err != nil {
			t.Fatal(err)
		}
		got := next.last()
		if got.Message != "ready" {
			t.Fatalf("message = %q; want ready", got.Message)
		}
		if attrBool(got, truncatedKey) {
			t.Fatal("log.truncated = true; want false")
		}
		if attrString(got, "id") != "1" {
			t.Fatalf("id = %q; want 1", attrString(got, "id"))
		}
		if group := attrGroup(got, "request"); group["method"] != "GET" {
			t.Fatalf("request.method = %q; want GET", group["method"])
		}

		passthrough := newTruncateHandler(slog.DiscardHandler, 32)
		allocs := testing.AllocsPerRun(100, func() {
			_ = passthrough.Handle(context.Background(), record)
		})
		if allocs != 0 {
			t.Fatalf("unchanged Handle allocs = %v; want 0", allocs)
		}
	})
	t.Run("message", func(t *testing.T) {
		next := &captureHandler{}
		handler := newTruncateHandler(next, 4)
		record := slog.NewRecord(time.Now(), slog.LevelInfo, "abcdefgh", 0)
		if err := handler.Handle(context.Background(), record); err != nil {
			t.Fatal(err)
		}
		got := next.last()
		if got.Message != "abcd" {
			t.Fatalf("message = %q; want abcd", got.Message)
		}
		if !attrBool(got, truncatedKey) {
			t.Fatal("log.truncated = false; want true")
		}
	})
	t.Run("string attr", func(t *testing.T) {
		next := &captureHandler{}
		handler := newTruncateHandler(next, 4)
		record := slog.NewRecord(time.Now(), slog.LevelInfo, "ok", 0)
		record.AddAttrs(slog.String("body", "abcdefgh"))
		if err := handler.Handle(context.Background(), record); err != nil {
			t.Fatal(err)
		}
		if got := attrString(next.last(), "body"); got != "abcd" {
			t.Fatalf("body = %q; want abcd", got)
		}
		if !attrBool(next.last(), truncatedKey) {
			t.Fatal("log.truncated = false; want true")
		}
	})
	t.Run("byte slice attr", func(t *testing.T) {
		next := &captureHandler{}
		handler := newTruncateHandler(next, 4)
		input := []byte{1, 2, 3, 4, 5, 6}
		record := slog.NewRecord(time.Now(), slog.LevelInfo, "ok", 0)
		record.AddAttrs(slog.Any("payload", input))
		if err := handler.Handle(context.Background(), record); err != nil {
			t.Fatal(err)
		}
		var got []byte
		next.last().Attrs(func(attr slog.Attr) bool {
			if attr.Key == "payload" {
				got, _ = attr.Value.Any().([]byte)
			}
			return true
		})
		if !bytes.Equal(got, []byte{1, 2, 3, 4}) {
			t.Fatalf("payload = %v; want [1 2 3 4]", got)
		}
		if !bytes.Equal(input, []byte{1, 2, 3, 4, 5, 6}) {
			t.Fatalf("input = %v; want unchanged", input)
		}
	})
	t.Run("byte slice exact length not truncated", func(t *testing.T) {
		next := &captureHandler{}
		handler := newTruncateHandler(next, 4)
		record := slog.NewRecord(time.Now(), slog.LevelInfo, "ok", 0)
		record.AddAttrs(slog.Any("payload", []byte{1, 2, 3, 4}))
		if err := handler.Handle(context.Background(), record); err != nil {
			t.Fatal(err)
		}
		if attrBool(next.last(), truncatedKey) {
			t.Fatal("log.truncated = true; want false")
		}
	})
	t.Run("non-byte any unchanged", func(t *testing.T) {
		next := &captureHandler{}
		handler := newTruncateHandler(next, 4)
		record := slog.NewRecord(time.Now(), slog.LevelInfo, "ok", 0)
		record.AddAttrs(slog.Any("count", 123456))
		if err := handler.Handle(context.Background(), record); err != nil {
			t.Fatal(err)
		}
		if attrBool(next.last(), truncatedKey) {
			t.Fatal("log.truncated = true; want false")
		}
	})
	t.Run("group truncates attrs after first change", func(t *testing.T) {
		next := &captureHandler{}
		handler := newTruncateHandler(next, 4)
		record := slog.NewRecord(time.Now(), slog.LevelInfo, "ok", 0)
		record.AddAttrs(slog.Group("request",
			slog.String("method", "GET"),
			slog.String("body", "abcdef"),
			slog.String("extra", "uvwxyz"),
		))
		if err := handler.Handle(context.Background(), record); err != nil {
			t.Fatal(err)
		}
		group := attrGroup(next.last(), "request")
		if group["method"] != "GET" || group["body"] != "abcd" || group["extra"] != "uvwx" {
			t.Fatalf("request = %v; want method=GET body=abcd extra=uvwx", group)
		}
		if !attrBool(next.last(), truncatedKey) {
			t.Fatal("log.truncated = false; want true")
		}
	})
	t.Run("group preserves resolved LogValuer", func(t *testing.T) {
		next := &captureHandler{}
		handler := newTruncateHandler(next, 8)
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
	t.Run("record truncation does not stick", func(t *testing.T) {
		logger, output := newJSONLogger(WithTruncate(4))
		logger.Info("abcdefgh")
		logger.Info("ok")
		lines := jsonLines(t, output.Bytes())
		if len(lines) != 2 {
			t.Fatalf("len(lines) = %d; want 2; output = %q", len(lines), output.String())
		}
		if lines[0][truncatedKey] != true {
			t.Fatalf("lines[0].%s = %v; want true", truncatedKey, lines[0][truncatedKey])
		}
		if _, ok := lines[1][truncatedKey]; ok {
			t.Fatalf("lines[1].%s = %v; want absent", truncatedKey, lines[1][truncatedKey])
		}
	})
}

func TestTruncateHandler_WithAttrsKeepsSingleTruncatedKey(t *testing.T) {
	logger, output := newJSONLogger(WithTruncate(4))
	logger = logger.With(slog.String("body", "abcdef"))
	logger.Info("ok")
	logger.Info("ok")
	lines := jsonLines(t, output.Bytes())
	if len(lines) != 2 {
		t.Fatalf("len(lines) = %d; want 2; output = %q", len(lines), output.String())
	}
	rawLines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(rawLines) != 2 {
		t.Fatalf("raw line count = %d; want 2", len(rawLines))
	}
	for i, line := range lines {
		if line["body"] != "abcd" {
			t.Fatalf("lines[%d].body = %v; want abcd", i, line["body"])
		}
		if line[truncatedKey] != true {
			t.Fatalf("lines[%d].%s = %v; want true", i, truncatedKey, line[truncatedKey])
		}
		if got := strings.Count(rawLines[i], `"log.truncated"`); got != 1 {
			t.Fatalf("lines[%d] log.truncated count = %d; want 1; line = %q", i, got, rawLines[i])
		}
	}
}

func TestTruncateAttr_UnchangedGroupDoesNotReportChange(t *testing.T) {
	handler := &truncateHandler{maxBytes: 8}
	attr := slog.Group("request", slog.String("method", "GET"))
	got, changed, truncated := handler.truncateAttr(attr)
	if changed || truncated {
		t.Fatalf("truncateAttr() changed, truncated = %v, %v; want false, false", changed, truncated)
	}
	if group := got.Value.Group(); len(group) != 1 || group[0].Value.String() != "GET" {
		t.Fatalf("truncateAttr() group = %v; want method=GET", group)
	}
}

func TestTruncateUTF8(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		maxBytes int
		want     string
		wantCut  bool
	}{
		{name: "empty", value: "", maxBytes: 4, want: ""},
		{name: "short", value: "ab", maxBytes: 8, want: "ab"},
		{name: "exact", value: "abcd", maxBytes: 4, want: "abcd"},
		{name: "ascii", value: "abcdefgh", maxBytes: 4, want: "abcd", wantCut: true},
		{name: "utf8 boundary", value: "你好世界", maxBytes: 6, want: "你好", wantCut: true},
		{name: "utf8 partial rune", value: "你好世界", maxBytes: 4, want: "你", wantCut: true},
		{name: "utf8 before first rune", value: "你", maxBytes: 2, want: "", wantCut: true},
		{name: "zero max", value: "ab", maxBytes: 0, want: "", wantCut: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, cut := truncateUTF8(tt.value, tt.maxBytes)
			if got != tt.want || cut != tt.wantCut {
				t.Errorf("truncateUTF8(%q, %d) = %q, %v; want %q, %v",
					tt.value, tt.maxBytes, got, cut, tt.want, tt.wantCut)
			}
			if cut && !utf8.ValidString(got) {
				t.Errorf("truncateUTF8(%q, %d) = %q; want valid UTF-8", tt.value, tt.maxBytes, got)
			}
		})
	}
}
