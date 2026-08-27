package log

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
)

func newJSONLogger(options ...Option) (*slog.Logger, *bytes.Buffer) {
	var output bytes.Buffer
	handler := NewHandler(append([]Option{
		WithWriter(&output),
		WithFormat(FormatJSON),
		WithLevel(slog.LevelDebug),
	}, options...)...)
	return slog.New(handler), &output
}

func mustPanic(t *testing.T, name string, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatalf("%s panic = nil; want panic", name)
		}
	}()
	fn()
}

type failWriter struct{}

func (failWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}

type panicWriter struct{}

func (panicWriter) Write([]byte) (int, error) {
	panic("boom")
}

type syncBuffer struct {
	mu     sync.Mutex
	output bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.output.Write(p)
}

func (b *syncBuffer) bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	data := make([]byte, b.output.Len())
	copy(data, b.output.Bytes())
	return data
}

type captureHandler struct {
	mu      sync.Mutex
	records []slog.Record
}

func (h *captureHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *captureHandler) Handle(_ context.Context, record slog.Record) error {
	h.mu.Lock()
	h.records = append(h.records, record.Clone())
	h.mu.Unlock()
	return nil
}

func (h *captureHandler) WithAttrs([]slog.Attr) slog.Handler { return h }

func (h *captureHandler) WithGroup(string) slog.Handler { return h }

func (h *captureHandler) last() slog.Record {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.records[len(h.records)-1]
}

type countingLogValuer struct {
	calls *int
	value slog.Value
}

func (valuer countingLogValuer) LogValue() slog.Value {
	(*valuer.calls)++
	return valuer.value
}

func attrString(record slog.Record, key string) string {
	var value string
	record.Attrs(func(attr slog.Attr) bool {
		if attr.Key == key {
			value = attr.Value.String()
		}
		return true
	})
	return value
}

func attrBool(record slog.Record, key string) bool {
	var value bool
	record.Attrs(func(attr slog.Attr) bool {
		if attr.Key == key {
			value = attr.Value.Bool()
		}
		return true
	})
	return value
}

func attrGroup(record slog.Record, key string) map[string]string {
	out := map[string]string{}
	record.Attrs(func(attr slog.Attr) bool {
		if attr.Key != key {
			return true
		}
		for _, groupAttr := range attr.Value.Group() {
			out[groupAttr.Key] = groupAttr.Value.String()
		}
		return true
	})
	return out
}

func attrUint64(record slog.Record, key string) uint64 {
	var value uint64
	record.Attrs(func(attr slog.Attr) bool {
		if attr.Key == key {
			value = attr.Value.Uint64()
		}
		return true
	})
	return value
}

func mustJSONObject(t *testing.T, data []byte) map[string]any {
	t.Helper()
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v; data = %q", err, data)
	}
	return got
}

func jsonLines(t *testing.T, data []byte) []map[string]any {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(data))
	var lines []map[string]any
	for {
		var line map[string]any
		err := decoder.Decode(&line)
		if errors.Is(err, io.EOF) {
			return lines
		}
		if err != nil {
			t.Fatalf("json.Decode() error = %v; data = %q", err, data)
		}
		lines = append(lines, line)
	}
}

var _ io.Writer = failWriter{}
var _ slog.Handler = (*captureHandler)(nil)
