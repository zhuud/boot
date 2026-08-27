package log

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/trace"

	"github.com/zhuud/boot/log/extractor/otel"
)

func restoreSlogDefault(t *testing.T) {
	t.Helper()
	previous := slog.Default()
	t.Cleanup(func() { slog.SetDefault(previous) })
}

func TestSetSlogDefault_AppliesRecommendedDefaults(t *testing.T) {
	restoreSlogDefault(t)
	var output bytes.Buffer
	SetSlogDefault("helloworld", "dev", WithWriter(&output))

	slog.Info("ok",
		slog.String("user_id", "u-1"),
		slog.String("password", "secret"),
		slog.String("token", "tok"),
		slog.String("body", strings.Repeat("a", 2048+1)),
	)

	got := mustJSONObject(t, output.Bytes())
	if got["msg"] != "ok" {
		t.Fatalf("msg = %v; want ok", got["msg"])
	}
	if got["service"] != "helloworld" {
		t.Fatalf("service = %v; want helloworld", got["service"])
	}
	if got["env"] != "dev" {
		t.Fatalf("env = %v; want dev", got["env"])
	}
	if got["log.truncated"] != true {
		t.Fatalf("log.truncated = %v; want true", got["log.truncated"])
	}
	if _, ok := got[slog.SourceKey]; !ok {
		t.Fatalf("source = %v; want object", got[slog.SourceKey])
	}
	if _, ok := got["user_id"]; ok {
		t.Fatalf("user_id at root = %v; want absent", got["user_id"])
	}
	attrs, ok := got["attrs"].(map[string]any)
	if !ok {
		t.Fatalf("attrs = %v; want object", got["attrs"])
	}
	if attrs["user_id"] != "u-1" {
		t.Fatalf("attrs.user_id = %v; want u-1", attrs["user_id"])
	}
	if attrs["password"] != "***" {
		t.Fatalf("attrs.password = %v; want ***", attrs["password"])
	}
	if attrs["token"] != "***" {
		t.Fatalf("attrs.token = %v; want ***", attrs["token"])
	}
	if got, ok := attrs["body"].(string); !ok || got != strings.Repeat("a", 2048) {
		t.Fatalf("attrs.body = %v; want %d a's", attrs["body"], 2048)
	}
}

func TestSetSlogDefault_WritesEmptyServiceAndEnv(t *testing.T) {
	restoreSlogDefault(t)
	var output bytes.Buffer
	SetSlogDefault("", "", WithWriter(&output))
	slog.Info("ok")

	got := mustJSONObject(t, output.Bytes())
	if got["service"] != "" {
		t.Fatalf("service = %v; want empty string", got["service"])
	}
	if got["env"] != "" {
		t.Fatalf("env = %v; want empty string", got["env"])
	}
}

func TestSetSlogDefault_SamplesRepeatedErrors(t *testing.T) {
	restoreSlogDefault(t)
	var output bytes.Buffer
	SetSlogDefault("helloworld", "dev", WithWriter(&output))

	total := 1000 + 1
	for range total {
		slog.Error("storm")
	}

	lines := jsonLines(t, output.Bytes())
	if got := len(lines); got != 1000 {
		t.Fatalf("lines = %d; want %d", got, 1000)
	}
}

func TestSetSlogDefault_DoesNotExtractTraceByDefault(t *testing.T) {
	restoreSlogDefault(t)
	var output bytes.Buffer
	SetSlogDefault("helloworld", "dev", WithWriter(&output))

	traceID, err := trace.TraceIDFromHex("0102030405060708090a0b0c0d0e0f10")
	if err != nil {
		t.Fatal(err)
	}
	spanID, err := trace.SpanIDFromHex("0102030405060708")
	if err != nil {
		t.Fatal(err)
	}
	ctx := trace.ContextWithSpanContext(context.Background(), trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
	}))
	slog.InfoContext(ctx, "handled")

	got := mustJSONObject(t, output.Bytes())
	if _, ok := got["trace_id"]; ok {
		t.Fatalf("trace_id = %v; want absent", got["trace_id"])
	}
	if _, ok := got["span_id"]; ok {
		t.Fatalf("span_id = %v; want absent", got["span_id"])
	}
}

func TestSetSlogDefault_ExtractsTrace(t *testing.T) {
	restoreSlogDefault(t)
	var output bytes.Buffer
	SetSlogDefault("helloworld", "dev", WithWriter(&output), WithContextExtractor(otel.TraceAttrsFromContext))

	traceID, err := trace.TraceIDFromHex("0102030405060708090a0b0c0d0e0f10")
	if err != nil {
		t.Fatal(err)
	}
	spanID, err := trace.SpanIDFromHex("0102030405060708")
	if err != nil {
		t.Fatal(err)
	}
	ctx := trace.ContextWithSpanContext(context.Background(), trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
	}))
	slog.InfoContext(ctx, "handled")

	got := mustJSONObject(t, output.Bytes())
	if got["trace_id"] != traceID.String() {
		t.Fatalf("trace_id = %v; want %s", got["trace_id"], traceID.String())
	}
	if got["span_id"] != spanID.String() {
		t.Fatalf("span_id = %v; want %s", got["span_id"], spanID.String())
	}
}

func TestSetSlogDefault_LaterOptionsOverrideDefaults(t *testing.T) {
	restoreSlogDefault(t)
	var output bytes.Buffer
	SetSlogDefault("helloworld", "dev",
		WithWriter(&output),
		WithFormat(FormatText),
		WithAttrGroup(""),
		WithTruncate(0),
	)
	slog.Info("ok",
		slog.String("user_id", "u-1"),
		slog.String("password", "secret"),
		slog.String("body", strings.Repeat("a", 2048+1)),
	)

	got := output.String()
	if !strings.Contains(got, "msg=ok") {
		t.Fatalf("output = %q; want text msg=ok", got)
	}
	if !strings.Contains(got, "user_id=u-1") {
		t.Fatalf("output = %q; want user_id at root", got)
	}
	if strings.Contains(got, "secret") {
		t.Fatalf("output = %q; want password redacted", got)
	}
	if !strings.Contains(got, strings.Repeat("a", 2048+1)) {
		t.Fatalf("output = %q; want untruncated body", got)
	}
	if strings.Contains(got, truncatedKey) {
		t.Fatalf("output = %q; want no truncation marker", got)
	}
}
