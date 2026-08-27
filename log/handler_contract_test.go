package log

import (
	"context"
	"log/slog"
	"testing"
	"time"
)

func TestDecorator_EnabledAndEmptyWith(t *testing.T) {
	disabled := slog.DiscardHandler
	enabled := &captureHandler{}
	tests := []struct {
		name            string
		disabledHandler slog.Handler
		enabledHandler  slog.Handler
	}{
		{
			name:            "attrGroup",
			disabledHandler: newAttrGroupHandler(disabled, "arg"),
			enabledHandler:  newAttrGroupHandler(enabled, "arg"),
		},
		{
			name:            "context",
			disabledHandler: newContextHandler(disabled, attrsFromContext),
			enabledHandler:  newContextHandler(enabled, attrsFromContext),
		},
		{
			name:            "redact",
			disabledHandler: newRedactHandler(disabled, "password"),
			enabledHandler:  newRedactHandler(enabled, "password"),
		},
		{
			name:            "drop",
			disabledHandler: newDropHandler(disabled, func(context.Context, slog.Record) bool { return false }),
			enabledHandler:  newDropHandler(enabled, func(context.Context, slog.Record) bool { return false }),
		},
		{
			name:            "sampling",
			disabledHandler: newSamplingHandler(disabled, SamplingConfig{Interval: time.Second}),
			enabledHandler:  newSamplingHandler(enabled, SamplingConfig{Interval: time.Second}),
		},
		{
			name:            "truncate",
			disabledHandler: newTruncateHandler(disabled, 8),
			enabledHandler:  newTruncateHandler(enabled, 8),
		},
		{
			name:            "error",
			disabledHandler: newErrorHandler(disabled, func(context.Context, slog.Record, error) {}),
			enabledHandler:  newErrorHandler(enabled, func(context.Context, slog.Record, error) {}),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.disabledHandler.Enabled(context.Background(), slog.LevelInfo); got {
				t.Fatal("Enabled() = true; want false")
			}
			if got := tt.enabledHandler.Enabled(context.Background(), slog.LevelInfo); !got {
				t.Fatal("Enabled() = false; want true")
			}
			if got := tt.enabledHandler.WithAttrs(nil); got != tt.enabledHandler {
				t.Fatalf("WithAttrs(nil) = %T; want same handler", got)
			}
			if got := tt.enabledHandler.WithAttrs([]slog.Attr{}); got != tt.enabledHandler {
				t.Fatalf("WithAttrs([]) = %T; want same handler", got)
			}
			if got := tt.enabledHandler.WithGroup(""); got != tt.enabledHandler {
				t.Fatalf("WithGroup(%q) = %T; want same handler", "", got)
			}
		})
	}
}
