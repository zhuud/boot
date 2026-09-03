//go:build unix

package lifecycle

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

const listenChildEnv = "BOOT_LIFECYCLE_LISTEN"

func TestListen_RunsCleanupOnSignal(t *testing.T) {
	if os.Getenv(listenChildEnv) == "1" {
		runListenChild(syscall.SIGUSR1, false)
	}
	runListenParent(t, syscall.SIGUSR1)
}

func TestListen_SecondCallIgnored(t *testing.T) {
	if os.Getenv(listenChildEnv) == "1" {
		runListenChild(syscall.SIGUSR1, true)
	}
	runListenParent(t, syscall.SIGUSR1)
}

func runListenChild(sig syscall.Signal, secondListenIgnored bool) {
	SetTimeout(0)
	SetPhaseDelay(0)
	var runs atomic.Int32
	Register(func() error {
		runs.Add(1)
		return nil
	})
	Listen(sig)
	if secondListenIgnored {
		Listen(syscall.SIGUSR2)
	}
	fmt.Println("ready")
	<-Done()
	if runs.Load() != 1 {
		os.Exit(1)
	}
	os.Exit(0)
}

func runListenParent(t *testing.T, sig syscall.Signal) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^"+t.Name()+"$")
	cmd.Env = append(os.Environ(), listenChildEnv+"=1")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	defer func() { _ = cmd.Process.Kill() }()

	ready := make(chan error, 1)
	go func() {
		r := bufio.NewReader(stdout)
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				ready <- err
				return
			}
			if line == "ready\n" {
				ready <- nil
				return
			}
		}
	}()
	select {
	case err := <-ready:
		if err != nil {
			t.Fatal(err)
		}
	case err := <-exited:
		t.Fatalf("child exited before ready: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("child not ready")
	}

	if err := cmd.Process.Signal(sig); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-exited:
		if err != nil {
			t.Fatalf("child: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("child did not exit after signal")
	}
}
