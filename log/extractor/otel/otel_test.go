package otel

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

func TestTraceAttrsFromContext(t *testing.T) {
	traceID, err := trace.TraceIDFromHex("0102030405060708090a0b0c0d0e0f10")
	if err != nil {
		t.Fatal(err)
	}
	spanID, err := trace.SpanIDFromHex("0102030405060708")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		ctx     context.Context
		wantLen int
		wantID  bool
	}{
		{name: "nil", ctx: nil},
		{name: "background", ctx: context.Background()},
		{
			name: "valid sampled",
			ctx: trace.ContextWithSpanContext(context.Background(), trace.NewSpanContext(trace.SpanContextConfig{
				TraceID:    traceID,
				SpanID:     spanID,
				TraceFlags: trace.FlagsSampled,
			})),
			wantLen: 2,
			wantID:  true,
		},
		{
			name: "valid unsampled",
			ctx: trace.ContextWithSpanContext(context.Background(), trace.NewSpanContext(trace.SpanContextConfig{
				TraceID: traceID,
				SpanID:  spanID,
			})),
			wantLen: 2,
			wantID:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TraceAttrsFromContext(tt.ctx)
			if len(got) != tt.wantLen {
				t.Fatalf("len(attrs) = %d; want %d", len(got), tt.wantLen)
			}
			if !tt.wantID {
				if got != nil {
					t.Fatalf("attrs = %v; want nil", got)
				}
				return
			}
			if got[0].Key != "trace_id" || got[0].Value.String() != traceID.String() {
				t.Fatalf("trace_id = %q; want %q", got[0].Value.String(), traceID.String())
			}
			if got[1].Key != "span_id" || got[1].Value.String() != spanID.String() {
				t.Fatalf("span_id = %q; want %q", got[1].Value.String(), spanID.String())
			}
		})
	}
}
