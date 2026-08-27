package log

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"
)

const (
	samplingLevelCount      = 2
	samplingBucketsPerLevel = 16384
	samplingSuppressedKey   = "log.suppressed"
)

// SamplingConfig 为共享相同 message 的精确 Warn 和 Error 记录配置采样。
//
// 只作用于 slog.LevelWarn 与 slog.LevelError，其他级别直接放行。
// 每个启用的 Handler 占用约 768 KiB（两级各 16384 个 message 哈希桶），碰撞会共享计数。
// 放行记录用 slog.Uint64("log.suppressed", n) 报告自上次放行以来累计抑制的条数。
// 并发时该计数可能顺延到后一条放行记录，不会重复；窗口切换可能轻微多采或少采。
type SamplingConfig struct {
	// Interval 是采样窗口。非正值禁用采样。
	Interval time.Duration
	// Initial 是每个 interval 内放行的匹配记录数。
	Initial uint64
	// Thereafter 在 Initial 之后每 N 条放行一次。零表示丢弃该 interval 内其余
	// 匹配记录。
	Thereafter uint64
}

type samplingCounter struct {
	resetAt    atomic.Int64
	count      atomic.Uint64
	suppressed atomic.Uint64
}

type samplingCounters [samplingLevelCount][samplingBucketsPerLevel]samplingCounter

type samplingHandler struct {
	next       slog.Handler
	interval   time.Duration
	initial    uint64
	thereafter uint64
	counters   *samplingCounters
}

func newSamplingHandler(next slog.Handler, sampling SamplingConfig) slog.Handler {
	if next == nil {
		panic("nil log handler")
	}
	if sampling.Interval <= 0 {
		return next
	}
	return &samplingHandler{
		next:       next,
		interval:   sampling.Interval,
		initial:    sampling.Initial,
		thereafter: sampling.Thereafter,
		counters:   new(samplingCounters),
	}
}

func (h *samplingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *samplingHandler) Handle(ctx context.Context, record slog.Record) error {
	counter, ok := h.counters.counter(record.Level, record.Message)
	if !ok {
		return h.next.Handle(ctx, record)
	}

	count := counter.nextCount(record.Time, h.interval)
	if count > h.initial &&
		(h.thereafter == 0 || (count-h.initial)%h.thereafter != 0) {
		counter.suppressed.Add(1)
		return nil
	}

	suppressed := counter.suppressed.Swap(0)
	if suppressed > 0 {
		record = record.Clone()
		record.AddAttrs(slog.Uint64(samplingSuppressedKey, suppressed))
	}
	return h.next.Handle(ctx, record)
}

func (h *samplingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	clone := *h
	clone.next = h.next.WithAttrs(attrs)
	return &clone
}

func (h *samplingHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	clone := *h
	clone.next = h.next.WithGroup(name)
	return &clone
}

func (counters *samplingCounters) counter(level slog.Level, message string) (*samplingCounter, bool) {
	var levelIndex int
	switch level {
	case slog.LevelWarn:
		levelIndex = 0
	case slog.LevelError:
		levelIndex = 1
	default:
		return nil, false
	}
	messageIndex := hashMessage(message) % samplingBucketsPerLevel
	return &counters[levelIndex][messageIndex], true
}

func (c *samplingCounter) nextCount(at time.Time, interval time.Duration) uint64 {
	timestamp := at.UnixNano()
	resetAt := c.resetAt.Load()
	if resetAt > timestamp {
		return c.count.Add(1)
	}

	c.count.Store(1)
	if !c.resetAt.CompareAndSwap(resetAt, timestamp+interval.Nanoseconds()) {
		// 另一个调用方并发重置了窗口，并且也 Store 了 1。
		return c.count.Add(1)
	}
	return 1
}

// hashMessage 按字节计算 FNV-1a。热路径不转 []byte，也不用 hash/fnv（会分配）。
func hashMessage(message string) uint32 {
	const (
		fnvOffset32 = 2166136261
		fnvPrime32  = 16777619
	)
	hash := uint32(fnvOffset32)
	for i := range len(message) {
		hash ^= uint32(message[i])
		hash *= fnvPrime32
	}
	return hash
}
