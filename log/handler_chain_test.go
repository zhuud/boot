package log

import (
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestNewHandler_AttrGroupKeepsSystemFieldsAtRoot(t *testing.T) {
	logger, output := newJSONLogger(
		WithSource(true),
		WithAttrGroup("arg"),
		WithTruncate(4),
		WithSampling(SamplingConfig{Interval: time.Hour, Initial: 1, Thereafter: 2}),
	)
	logger.Error("storm", slog.String("body", "abcdef"))
	logger.Error("storm", slog.String("body", "abcdef"))
	logger.Error("storm", slog.String("body", "abcdef"))

	lines := jsonLines(t, output.Bytes())
	if len(lines) != 2 {
		t.Fatalf("len(lines) = %d; want 2; output = %q", len(lines), output.String())
	}
	for i, line := range lines {
		if _, ok := line[slog.SourceKey]; !ok {
			t.Fatalf("lines[%d].source = %v; want root source", i, line[slog.SourceKey])
		}
		if line[truncatedKey] != true {
			t.Fatalf("lines[%d].%s = %v; want true", i, truncatedKey, line[truncatedKey])
		}
		arg, ok := line["arg"].(map[string]any)
		if !ok || arg["body"] != "abcd" {
			t.Fatalf("lines[%d].arg = %v; want body=abcd", i, line["arg"])
		}
		if _, ok := arg[slog.SourceKey]; ok {
			t.Fatalf("lines[%d].arg.source = %v; want absent", i, arg[slog.SourceKey])
		}
		if _, ok := arg[truncatedKey]; ok {
			t.Fatalf("lines[%d].arg.%s = %v; want absent", i, truncatedKey, arg[truncatedKey])
		}
	}
	if got := lines[1][samplingSuppressedKey]; got != float64(1) {
		t.Fatalf("second %s = %v; want 1", samplingSuppressedKey, got)
	}
}

func TestNewHandler_DropFuncSeesContextAndGroupedCallAttrsNotWithAttrs(t *testing.T) {
	var seen []string
	logger, output := newJSONLogger(
		WithAttrGroup("arg"),
		WithDropFunc(func(_ context.Context, record slog.Record) bool {
			record.Attrs(func(attr slog.Attr) bool {
				seen = append(seen, attr.Key)
				return true
			})
			return false
		}),
	)
	logger = logger.With(slog.String("service", "api"))
	ctx := ContextWithAttrs(context.Background(), slog.String("request_id", "r1"))
	logger.InfoContext(ctx, "ready", slog.String("user_id", "u-1"))

	if len(seen) != 2 || seen[0] != "arg" || seen[1] != "request_id" {
		t.Fatalf("drop keys = %v; want [arg request_id]", seen)
	}
	got := mustJSONObject(t, output.Bytes())
	if got["service"] != "api" {
		t.Fatalf("service = %v; want api", got["service"])
	}
	if got["request_id"] != "r1" {
		t.Fatalf("request_id = %v; want r1", got["request_id"])
	}
	arg, ok := got["arg"].(map[string]any)
	if !ok || arg["user_id"] != "u-1" {
		t.Fatalf("arg = %v; want user_id=u-1", got["arg"])
	}
}

func TestNewHandler_DropFuncSeesRedactedValue(t *testing.T) {
	var seen string
	next := &captureHandler{}
	handler := newRedactHandler(newDropHandler(next, func(_ context.Context, record slog.Record) bool {
		record.Attrs(func(attr slog.Attr) bool {
			if attr.Key == "password" {
				seen = attr.Value.String()
			}
			return true
		})
		return false
	}), "password")
	record := slog.NewRecord(time.Now(), slog.LevelInfo, "login", 0)
	record.AddAttrs(slog.String("password", "secret"))
	if err := handler.Handle(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	if seen != redactedValue {
		t.Fatalf("drop saw password = %q; want %q", seen, redactedValue)
	}
	if got := attrString(next.last(), "password"); got != redactedValue {
		t.Fatalf("downstream password = %q; want %q", got, redactedValue)
	}
}

func TestNewHandler_WithAndContextKeepDuplicateKeys(t *testing.T) {
	logger, output := newJSONLogger()
	logger = logger.With(slog.String("logger_key", "logger"))
	ctx := ContextWithAttrs(context.Background(), slog.String("logger_key", "context"))
	logger.InfoContext(ctx, "handled")

	if count := strings.Count(output.String(), `"logger_key":`); count != 2 {
		t.Fatalf("logger_key count in %q = %d; want 2", output.String(), count)
	}
	got := mustJSONObject(t, output.Bytes())
	if got["logger_key"] != "context" {
		t.Fatalf("unmarshaled logger_key = %v; want last-win context", got["logger_key"])
	}
}

func TestNewHandler_DerivedLoggerPreservesDecoratorSemantics(t *testing.T) {
	dropCalls := 0
	errorCalls := 0
	logger, output := newJSONLogger(
		WithSource(true),
		WithAttrGroup("arg"),
		WithRedactKey("request.password"),
		WithDropFunc(func(context.Context, slog.Record) bool {
			dropCalls++
			return false
		}),
		WithSampling(SamplingConfig{Interval: time.Hour, Initial: 10, Thereafter: 1}),
		WithTruncate(4),
		WithErrorFunc(func(context.Context, slog.Record, error) {
			errorCalls++
		}),
	)
	logger = logger.WithGroup("request").With(
		slog.String("password", "secret"),
		slog.String("body", "abcdef"),
	)
	ctx := ContextWithAttrs(context.Background(), slog.String("request_id", "r1"))
	logger.InfoContext(ctx, "ready", slog.String("item", "abcdef"))

	got := mustJSONObject(t, output.Bytes())
	if _, ok := got[slog.SourceKey]; !ok {
		t.Fatalf("source = %v; want root source", got[slog.SourceKey])
	}
	request, ok := got["request"].(map[string]any)
	if !ok {
		t.Fatalf("request = %v; want object", got["request"])
	}
	if request["password"] != redactedValue {
		t.Fatalf("request.password = %v; want %q", request["password"], redactedValue)
	}
	if request["body"] != "abcd" {
		t.Fatalf("request.body = %v; want abcd", request["body"])
	}
	if request[truncatedKey] != true {
		t.Fatalf("request.%s = %v; want true", truncatedKey, request[truncatedKey])
	}
	if request["request_id"] != "r1" {
		t.Fatalf("request.request_id = %v; want r1", request["request_id"])
	}
	arg, ok := request["arg"].(map[string]any)
	if !ok || arg["item"] != "abcd" {
		t.Fatalf("request.arg = %v; want item=abcd", request["arg"])
	}
	if dropCalls != 1 {
		t.Fatalf("drop calls = %d; want 1", dropCalls)
	}
	if errorCalls != 0 {
		t.Fatalf("error calls = %d; want 0", errorCalls)
	}
}

func TestNewHandler_DerivedLoggersShareSamplingState(t *testing.T) {
	logger, output := newJSONLogger(WithSampling(SamplingConfig{
		Interval:   time.Hour,
		Initial:    1,
		Thereafter: 0,
	}))
	logger.Warn("storm")
	logger.With(slog.String("scope", "attrs")).Warn("storm")
	logger.WithGroup("scope").Warn("storm")

	if got := strings.Count(output.String(), `"msg":"storm"`); got != 1 {
		t.Fatalf("storm count = %d; want 1; output = %q", got, output.String())
	}
}

func TestNewHandler_DerivedLoggerReportsWriteError(t *testing.T) {
	calls := 0
	logger := slog.New(NewHandler(
		WithWriter(failWriter{}),
		WithFormat(FormatJSON),
		WithErrorFunc(func(context.Context, slog.Record, error) {
			calls++
		}),
	)).WithGroup("request").With(slog.String("id", "r1"))
	logger.Info("ready")
	if calls != 1 {
		t.Fatalf("error calls = %d; want 1", calls)
	}
}
