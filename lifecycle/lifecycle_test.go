package lifecycle

import (
	"errors"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

func reset(t *testing.T) {
	t.Helper()
	mu.Lock()
	registered = [phaseCount]phaseHooks{}
	done = make(chan struct{})
	cleanupOnce = sync.Once{}
	listenOnce = sync.Once{}
	stopping = false
	cleanupErr = nil
	timeout = 0
	phaseDelay = 0
	forceSignal = syscall.SIGTERM
	mu.Unlock()
}

func TestOptions(t *testing.T) {
	config := registerOptions{phase: phaseCleanup}

	Drain()(&config)
	if config.phase != phaseDrain {
		t.Fatalf("phase = %v; want phaseDrain", config.phase)
	}

	Async()(&config)
	if !config.async {
		t.Fatal("Async did not enable asynchronous execution")
	}
}

func TestRegisterIgnoresNilOption(t *testing.T) {
	reset(t)
	var ran atomic.Bool

	Register(func() error {
		ran.Store(true)
		return nil
	}, nil)

	if err := Cleanup(); err != nil {
		t.Fatal(err)
	}
	if !ran.Load() {
		t.Fatal("hook did not run")
	}
}

func TestCleanup_SyncLIFO(t *testing.T) {
	reset(t)
	var order []int
	Register(func() error { order = append(order, 1); return nil })
	Register(func() error { order = append(order, 2); return nil })
	Register(func() error { order = append(order, 3); return nil })

	if err := Cleanup(); err != nil {
		t.Fatal(err)
	}
	if len(order) != 3 || order[0] != 3 || order[1] != 2 || order[2] != 1 {
		t.Fatalf("order = %v; want [3 2 1]", order)
	}
}

func TestCleanup_DrainBeforeCleanup(t *testing.T) {
	reset(t)
	var order []int
	var orderMu sync.Mutex
	add := func(v int) {
		orderMu.Lock()
		order = append(order, v)
		orderMu.Unlock()
	}

	Register(func() error { add(1); return nil }, Drain())
	Register(func() error { add(2); return nil }, Drain())
	Register(func() error { add(3); return nil })
	Register(func() error {
		time.Sleep(20 * time.Millisecond)
		add(4)
		return nil
	}, Async())

	if err := Cleanup(); err != nil {
		t.Fatal(err)
	}
	orderMu.Lock()
	defer orderMu.Unlock()
	if len(order) != 4 || order[0] != 2 || order[1] != 1 {
		t.Fatalf("order = %v; want Drain prefix [2 1 ...]", order)
	}
}

func TestCleanup_AsyncParallel(t *testing.T) {
	reset(t)
	var n atomic.Int32
	block := make(chan struct{})

	Register(func() error {
		<-block
		n.Add(1)
		return nil
	}, Async())
	Register(func() error {
		<-block
		n.Add(1)
		return nil
	}, Async())
	Register(func() error {
		close(block)
		return nil
	})

	if err := Cleanup(); err != nil {
		t.Fatal(err)
	}
	if got := n.Load(); got != 2 {
		t.Fatalf("n = %d; want 2", got)
	}
}

func TestCleanup_Idempotent(t *testing.T) {
	reset(t)
	var n atomic.Int32
	Register(func() error { n.Add(1); return nil }, Drain())
	Register(func() error { n.Add(1); return nil })
	Register(func() error { n.Add(1); return nil }, Async())
	_ = Cleanup()
	_ = Cleanup()
	if got := n.Load(); got != 3 {
		t.Fatalf("n = %d; want 3", got)
	}
}

func TestCleanup_ConcurrentCallsRunHooksOnce(t *testing.T) {
	reset(t)
	var runs atomic.Int32
	Register(func() error {
		runs.Add(1)
		return nil
	})

	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = Cleanup()
		}()
	}
	wg.Wait()

	if got := runs.Load(); got != 1 {
		t.Fatalf("hook runs = %d; want 1", got)
	}
}

func TestRegister_ConcurrentWithCleanupDoesNotLoseHooks(t *testing.T) {
	reset(t)
	const count = 100

	var runs atomic.Int32
	start := make(chan struct{})
	var wg sync.WaitGroup
	for range count {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			Register(func() error {
				runs.Add(1)
				return nil
			})
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		_ = Cleanup()
	}()

	close(start)
	wg.Wait()

	if got := runs.Load(); got != count {
		t.Fatalf("hook runs = %d; want %d", got, count)
	}
}

func TestRegister_AfterCleanup(t *testing.T) {
	reset(t)
	_ = Cleanup()
	var ran atomic.Int32
	Register(func() error { ran.Add(1); return nil })
	Register(func() error { ran.Add(1); return nil }, Async())
	Register(func() error { ran.Add(1); return nil }, Drain())
	if got := ran.Load(); got != 3 {
		t.Fatalf("ran = %d; want 3", got)
	}
}

func TestRegisterCloser(t *testing.T) {
	reset(t)
	var closed atomic.Bool
	RegisterCloser(closerFunc(func() error { closed.Store(true); return nil }))
	RegisterCloser(closerFunc(func() error { return nil }), Async())
	_ = Cleanup()
	if !closed.Load() {
		t.Fatal("not closed")
	}
}

func TestCleanup_Errors(t *testing.T) {
	reset(t)
	a, b, c := errors.New("a"), errors.New("b"), errors.New("c")
	Register(func() error { return a }, Drain())
	Register(func() error { return b })
	Register(func() error { return c }, Async())
	err := Cleanup()
	if !errors.Is(err, a) || !errors.Is(err, b) || !errors.Is(err, c) {
		t.Fatalf("err = %v; want joined a,b,c", err)
	}
}

func TestCleanup_Panic(t *testing.T) {
	reset(t)
	Register(func() error { panic("x") })
	if Cleanup() == nil {
		t.Fatal("Cleanup() error = nil; want panic error")
	}
}

func TestDone(t *testing.T) {
	reset(t)
	select {
	case <-Done():
		t.Fatal("Done closed before Cleanup")
	default:
	}
	_ = Cleanup()
	select {
	case <-Done():
	default:
		t.Fatal("Done not closed after Cleanup")
	}
}

func TestCleanup_ConcurrentCallsReturnSameError(t *testing.T) {
	reset(t)
	want := errors.New("cleanup failed")
	started := make(chan struct{})
	release := make(chan struct{})
	Register(func() error {
		close(started)
		<-release
		return want
	})

	first := make(chan error, 1)
	second := make(chan error, 1)
	go func() { first <- Cleanup() }()
	<-started
	go func() { second <- Cleanup() }()
	close(release)

	for _, result := range []<-chan error{first, second} {
		if err := <-result; !errors.Is(err, want) {
			t.Fatalf("Cleanup error = %v; want %v", err, want)
		}
	}
}

func TestPhaseDelay_BetweenNonEmptyPhases(t *testing.T) {
	reset(t)
	SetPhaseDelay(30 * time.Millisecond)
	Register(func() error { return nil }, Drain())
	Register(func() error { return nil })

	start := time.Now()
	if err := Cleanup(); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(start); elapsed < 25*time.Millisecond {
		t.Fatalf("Cleanup elapsed %v; want phase delay", elapsed)
	}
}

func TestPhaseDelay_NonPositiveDisablesDelay(t *testing.T) {
	for _, delay := range []time.Duration{0, -time.Second} {
		t.Run(delay.String(), func(t *testing.T) {
			reset(t)
			SetPhaseDelay(delay)
			Register(func() error { return nil }, Drain())
			Register(func() error { return nil })

			start := time.Now()
			if err := Cleanup(); err != nil {
				t.Fatal(err)
			}
			if elapsed := time.Since(start); elapsed >= 50*time.Millisecond {
				t.Fatalf("Cleanup elapsed %v; want delay disabled", elapsed)
			}
		})
	}
}

type closerFunc func() error

func (f closerFunc) Close() error { return f() }
