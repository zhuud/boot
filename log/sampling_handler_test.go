package log

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"
)

func TestNewSamplingHandler_NilNextPanics(t *testing.T) {
	mustPanic(t, "newSamplingHandler()", func() {
		_ = newSamplingHandler(nil, SamplingConfig{Interval: time.Second})
	})
}

func TestNewSamplingHandler_DisabledReturnsNext(t *testing.T) {
	next := &captureHandler{}
	for _, interval := range []time.Duration{0, -time.Second} {
		if got := newSamplingHandler(next, SamplingConfig{Interval: interval}); got != next {
			t.Fatalf("Interval=%s = %T; want next", interval, got)
		}
	}
}

func TestSamplingHandler_Handle(t *testing.T) {
	t.Run("adds suppressed", func(t *testing.T) {
		next := &captureHandler{}
		handler := newSamplingHandler(next, SamplingConfig{
			Interval:   time.Hour,
			Initial:    1,
			Thereafter: 0,
		})
		now := time.Unix(1_700_000_000, 0)
		if err := handler.Handle(context.Background(), slog.NewRecord(now, slog.LevelWarn, "storm", 0)); err != nil {
			t.Fatal(err)
		}
		if err := handler.Handle(context.Background(), slog.NewRecord(now, slog.LevelWarn, "storm", 0)); err != nil {
			t.Fatal(err)
		}
		if err := handler.Handle(context.Background(), slog.NewRecord(now.Add(time.Hour), slog.LevelWarn, "storm", 0)); err != nil {
			t.Fatal(err)
		}
		if got := len(next.records); got != 2 {
			t.Fatalf("admitted = %d; want 2", got)
		}
		if got := attrUint64(next.records[1], samplingSuppressedKey); got != 1 {
			t.Fatalf("log.suppressed = %d; want 1", got)
		}
	})
	t.Run("thereafter and levels are independent", func(t *testing.T) {
		next := &captureHandler{}
		handler := newSamplingHandler(next, SamplingConfig{
			Interval:   time.Hour,
			Initial:    1,
			Thereafter: 2,
		})
		now := time.Unix(1_700_000_000, 0)
		for _, level := range []slog.Level{slog.LevelWarn, slog.LevelError} {
			for range 3 {
				if err := handler.Handle(context.Background(), slog.NewRecord(now, level, "storm", 0)); err != nil {
					t.Fatal(err)
				}
			}
		}
		if err := handler.Handle(context.Background(), slog.NewRecord(now, slog.LevelInfo, "storm", 0)); err != nil {
			t.Fatal(err)
		}
		if got := len(next.records); got != 5 {
			t.Fatalf("admitted = %d; want 5", got)
		}
		for _, index := range []int{1, 3} {
			if got := attrUint64(next.records[index], samplingSuppressedKey); got != 1 {
				t.Fatalf("records[%d] log.suppressed = %d; want 1", index, got)
			}
		}
		if next.records[0].Level != slog.LevelWarn || next.records[2].Level != slog.LevelError || next.records[4].Level != slog.LevelInfo {
			t.Fatalf("admitted levels = %v, %v, %v; want WARN, ERROR, INFO",
				next.records[0].Level, next.records[2].Level, next.records[4].Level)
		}
	})
	t.Run("drops all when initial and thereafter zero", func(t *testing.T) {
		next := &captureHandler{}
		handler := newSamplingHandler(next, SamplingConfig{
			Interval:   time.Hour,
			Initial:    0,
			Thereafter: 0,
		})
		if err := handler.Handle(context.Background(), slog.NewRecord(time.Unix(1_700_000_000, 0), slog.LevelWarn, "storm", 0)); err != nil {
			t.Fatal(err)
		}
		if got := len(next.records); got != 0 {
			t.Fatalf("admitted = %d; want 0", got)
		}
	})
	t.Run("different messages are independent", func(t *testing.T) {
		if hashMessage("storm-a")%samplingBucketsPerLevel == hashMessage("storm-b")%samplingBucketsPerLevel {
			t.Skip("messages hashed to the same sampling bucket")
		}
		next := &captureHandler{}
		handler := newSamplingHandler(next, SamplingConfig{
			Interval:   time.Hour,
			Initial:    1,
			Thereafter: 0,
		})
		now := time.Unix(1_700_000_000, 0)
		for _, message := range []string{"storm-a", "storm-a", "storm-b"} {
			if err := handler.Handle(context.Background(), slog.NewRecord(now, slog.LevelWarn, message, 0)); err != nil {
				t.Fatal(err)
			}
		}
		if got := len(next.records); got != 2 {
			t.Fatalf("admitted = %d; want 2", got)
		}
		if next.records[0].Message != "storm-a" || next.records[1].Message != "storm-b" {
			t.Fatalf("messages = %q, %q; want storm-a, storm-b", next.records[0].Message, next.records[1].Message)
		}
	})
}

func TestSamplingCounter_NextCount(t *testing.T) {
	var counter samplingCounter
	interval := time.Second
	window := time.Unix(1_700_000_000, 0)
	for i, want := range []uint64{1, 2, 3} {
		if got := counter.nextCount(window, interval); got != want {
			t.Fatalf("nextCount() call %d = %d; want %d", i+1, got, want)
		}
	}
	if got := counter.nextCount(window.Add(interval), interval); got != 1 {
		t.Fatalf("nextCount() after interval = %d; want 1", got)
	}
}

func TestSamplingCounter_ConcurrentFirstWindow(t *testing.T) {
	const workers = 256
	var counter samplingCounter
	interval := time.Second
	window := time.Unix(1_700_000_000, 0)
	start := make(chan struct{})
	counts := make([]uint64, workers)
	var wait sync.WaitGroup
	wait.Add(workers)
	for i := range workers {
		go func() {
			defer wait.Done()
			<-start
			counts[i] = counter.nextCount(window, interval)
		}()
	}
	close(start)
	wait.Wait()

	resetObserved := false
	for i, got := range counts {
		if got == 1 {
			resetObserved = true
		}
		if got == 0 || got > workers {
			t.Fatalf("counts[%d] = %d; want within [1,%d]; counts = %v", i, got, workers, counts)
		}
	}
	if !resetObserved {
		t.Fatalf("counts = %v; want a reset owner", counts)
	}
}

func TestSamplingCounter_ConcurrentResetProgress(t *testing.T) {
	const (
		workers = 64
		rounds  = 50
	)
	var counter samplingCounter
	interval := time.Second
	window := time.Unix(1_700_000_000, 0)
	if got := counter.nextCount(window, interval); got != 1 {
		t.Fatalf("initial nextCount() = %d; want 1", got)
	}

	for round := 0; round < rounds; round++ {
		window = window.Add(interval)
		start := make(chan struct{})
		counts := make([]uint64, workers)
		var wait sync.WaitGroup
		wait.Add(workers)
		for i := range workers {
			go func() {
				defer wait.Done()
				<-start
				counts[i] = counter.nextCount(window, interval)
			}()
		}
		close(start)
		wait.Wait()

		resetObserved := false
		for i, got := range counts {
			if got == 1 {
				resetObserved = true
			}
			if got == 0 || got > workers {
				t.Fatalf("round %d counts[%d] = %d; want within [1,%d]; counts = %v", round, i, got, workers, counts)
			}
		}
		if !resetObserved {
			t.Fatalf("round %d counts = %v; want a reset owner", round, counts)
		}
		if got := counter.nextCount(window, interval); got <= 1 || got > workers+1 {
			t.Fatalf("round %d nextCount() = %d; want within [2,%d]", round, got, workers+1)
		}
	}
}

func TestSamplingHandler_ConcurrentHandle(t *testing.T) {
	const workers = 64
	next := &captureHandler{}
	handler := newSamplingHandler(next, SamplingConfig{
		Interval:   time.Hour,
		Initial:    8,
		Thereafter: 0,
	})
	now := time.Unix(1_700_000_000, 0)
	start := make(chan struct{})
	var wait sync.WaitGroup
	wait.Add(workers)
	for range workers {
		go func() {
			defer wait.Done()
			<-start
			if err := handler.Handle(context.Background(), slog.NewRecord(now, slog.LevelWarn, "storm", 0)); err != nil {
				t.Errorf("Handle() error = %v; want nil", err)
			}
		}()
	}
	close(start)
	wait.Wait()
	if got := len(next.records); got < 1 || got > workers {
		t.Fatalf("admitted = %d; want within [1,%d]", got, workers)
	}
}
