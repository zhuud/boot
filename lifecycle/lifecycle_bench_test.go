package lifecycle

import (
	"sync"
	"syscall"
	"testing"
)

func resetForBench() {
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

func BenchmarkCleanup_SyncHooks(b *testing.B) {
	b.ReportAllocs()
	for range b.N {
		resetForBench()
		Register(func() error { return nil })
		Register(func() error { return nil })
		Register(func() error { return nil })
		if err := Cleanup(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCleanup_AsyncHooks(b *testing.B) {
	b.ReportAllocs()
	for range b.N {
		resetForBench()
		Register(func() error { return nil }, Async())
		Register(func() error { return nil }, Async())
		Register(func() error { return nil }, Drain(), Async())
		if err := Cleanup(); err != nil {
			b.Fatal(err)
		}
	}
}
