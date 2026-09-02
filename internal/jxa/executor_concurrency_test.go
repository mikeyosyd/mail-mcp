package jxa

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// stubSuccess is the minimal JSON a well-behaved JXA script prints.
const stubSuccess = `{"success":true,"data":{"ok":true}}`

// stubRunner replaces runOSAScript with fn for the duration of the test, so
// Execute can be exercised without spawning osascript.
func stubRunner(t *testing.T, fn func(ctx context.Context, args ...string) ([]byte, error)) {
	t.Helper()
	orig := runOSAScript
	runOSAScript = fn
	t.Cleanup(func() { runOSAScript = orig })
}

func TestExecute_SerialisesConcurrentCalls(t *testing.T) {
	const calls = 8
	const hold = 20 * time.Millisecond

	type interval struct{ start, end time.Time }
	var (
		mu   sync.Mutex
		runs []interval
	)
	stubRunner(t, func(ctx context.Context, args ...string) ([]byte, error) {
		start := time.Now()
		time.Sleep(hold) // long enough for unserialised calls to overlap
		end := time.Now()

		mu.Lock()
		runs = append(runs, interval{start, end})
		mu.Unlock()
		return []byte(stubSuccess), nil
	})

	begin := time.Now()
	var wg sync.WaitGroup
	errs := make([]error, calls)
	for i := range calls {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = Execute(context.Background(), "script")
		}()
	}
	wg.Wait()
	elapsed := time.Since(begin)

	for i, err := range errs {
		if err != nil {
			t.Errorf("Execute() call %d error = %v, want nil", i, err)
		}
	}
	if len(runs) != calls {
		t.Fatalf("osascript runs = %d, want %d", len(runs), calls)
	}

	// No run may start before the previous one has ended.
	sort.Slice(runs, func(i, j int) bool { return runs[i].start.Before(runs[j].start) })
	for i := 1; i < len(runs); i++ {
		if runs[i].start.Before(runs[i-1].end) {
			t.Errorf("run %d started %v before run %d ended: osascript runs overlapped",
				i, runs[i-1].end.Sub(runs[i].start), i-1)
		}
	}

	// Serialised runs take at least the sum of their durations.
	if elapsed < calls*hold {
		t.Errorf("all %d calls finished in %v, want at least %v for serialised runs", calls, elapsed, calls*hold)
	}
}

func TestExecute_CancelledWhileWaitingForEarlierScript(t *testing.T) {
	var (
		entered     = make(chan struct{})
		enteredOnce sync.Once
		release     = make(chan struct{})
		releaseOnce sync.Once
		entries     atomic.Int32
	)
	stubRunner(t, func(ctx context.Context, args ...string) ([]byte, error) {
		entries.Add(1)
		enteredOnce.Do(func() { close(entered) })
		<-release
		return []byte(stubSuccess), nil
	})
	t.Cleanup(func() { releaseOnce.Do(func() { close(release) }) })

	// The first call holds the lock inside the runner until released.
	firstDone := make(chan error, 1)
	go func() {
		_, err := Execute(context.Background(), "script")
		firstDone <- err
	}()
	<-entered

	// The second call queues behind the first; its context expires while it waits.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	secondDone := make(chan error, 1)
	go func() {
		_, err := Execute(ctx, "script")
		secondDone <- err
	}()

	select {
	case err := <-secondDone:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("Execute() error = %v, want one wrapping context.DeadlineExceeded", err)
		}
		if err == nil || !strings.Contains(err.Error(), "waiting for an earlier JXA script") {
			t.Errorf("Execute() error should say it was waiting for an earlier script, got: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Execute() did not return after its context expired while waiting for the lock")
	}

	if got := entries.Load(); got != 1 {
		t.Errorf("osascript runs started = %d, want 1 (the waiting call must not start osascript)", got)
	}

	// Releasing the first call lets it finish normally.
	releaseOnce.Do(func() { close(release) })
	if err := <-firstDone; err != nil {
		t.Errorf("first Execute() error = %v, want nil", err)
	}
}

func TestExecute_ReleasesLockAfterFailedRun(t *testing.T) {
	stubRunner(t, func(ctx context.Context, args ...string) ([]byte, error) {
		return []byte("boom"), errors.New("exit status 1")
	})
	if _, err := Execute(context.Background(), "script"); err == nil {
		t.Fatal("Execute() error = nil, want error from failed run")
	}

	stubRunner(t, func(ctx context.Context, args ...string) ([]byte, error) {
		return []byte(stubSuccess), nil
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := Execute(ctx, "script"); err != nil {
		t.Fatalf("Execute() after a failed run error = %v, want nil (lock was not released)", err)
	}
}
