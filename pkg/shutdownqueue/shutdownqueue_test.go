package shutdownqueue

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// resetQueue clears the global queue between tests without fighting init/Once.
func resetQueue(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		q.mu.Lock()

		q.tasks = nil
		q.closed = false

		q.mu.Unlock()
	})
}

//nolint:paralleltest
func TestAddNilTaskError(t *testing.T) {
	resetQueue(t)

	// New behavior: Add(nil) is a no-op (no error).
	Add(nil)

	// Verify that Shutdown runs with no tasks and returns nil.
	err := Shutdown(t.Context())
	if err != nil {
		t.Fatalf("expected nil after adding nil task; got %v", err)
	}
}

//nolint:paralleltest
func TestLIFOOrder(t *testing.T) {
	resetQueue(t)

	var (
		orderMu sync.Mutex
		order   []int
	)

	makeTask := func(n int) Task {
		return func(ctx context.Context) error {
			orderMu.Lock()

			order = append(order, n)

			orderMu.Unlock()

			return nil
		}
	}

	for i := 1; i <= 3; i++ {
		Add(makeTask(i))
	}

	err := Shutdown(t.Context())
	if err != nil {
		t.Fatalf("Shutdown error: %v", err)
	}

	want := []int{3, 2, 1}
	if len(order) != len(want) {
		t.Fatalf("order len mismatch: got %v, want %v", order, want)
	}

	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("order mismatch at %d: got %v, want %v", i, order, want)
		}
	}
}

//nolint:paralleltest
func TestPanicRecoveryIncludedAndContinues(t *testing.T) {
	resetQueue(t)

	var ranAfterPanic atomic.Bool

	panicTask := func(ctx context.Context) error {
		panic("boom")
	}

	after := func(ctx context.Context) error {
		ranAfterPanic.Store(true)

		return nil
	}

	before := func(ctx context.Context) error { return nil }

	Add(before)
	Add(panicTask)
	Add(after)

	shErr := Shutdown(t.Context())
	if shErr == nil {
		t.Fatalf("expected aggregated error with panic; got nil")
	}

	if !strings.Contains(shErr.Error(), "panic in shutdown task: boom") {
		t.Fatalf("expected panic message in error; got: %q", shErr.Error())
	}

	if !ranAfterPanic.Load() {
		t.Fatalf("expected tasks after the panic to still run")
	}
}

//nolint:paralleltest
func TestAggregatedErrorsAndEarlyCancel(t *testing.T) {
	resetQueue(t)

	errA := errors.New("taskA")

	var ranB atomic.Bool

	taskA := func(ctx context.Context) error { return errA }
	taskB := func(ctx context.Context) error {
		ranB.Store(true)

		return nil
	}

	// Gate blocks until ctx is canceled. That ensures cancellation is active
	// before Shutdown proceeds to taskB.
	gateReady := make(chan struct{})
	gate := func(ctx context.Context) error {
		close(gateReady) // signal we've entered the gate
		<-ctx.Done()     // block until the test cancels

		return nil
	}

	Add(taskA)
	Add(taskB)
	Add(gate) // LIFO: gate, B, A

	// Use test-scoped context, wrap with cancel so we control when it ends.
	ctx, cancel := context.WithCancel(t.Context())
	errCh := make(chan error, 1)

	go func() {
		errCh <- Shutdown(ctx)
	}()

	// Wait until gate is running, then cancel so Shutdown stops before B.
	<-gateReady
	cancel()

	shErr := <-errCh
	if shErr == nil {
		t.Fatalf("expected error due to context cancel; got nil")
	}
	// Should include context cancellation.
	if !errors.Is(shErr, context.Canceled) {
		t.Fatalf("expected errors.Is(err, context.Canceled); got: %v", shErr)
	}
	// B must not have run; A must not have been reached.
	if ranB.Load() {
		t.Fatalf("expected taskB not to run after cancel")
	}

	if errors.Is(shErr, errA) {
		t.Fatalf("did not expect joined error to include taskA")
	}
}

//nolint:paralleltest
func TestIdempotentAndRunsOnce(t *testing.T) {
	resetQueue(t)

	var count atomic.Int32

	task := func(ctx context.Context) error {
		count.Add(1)

		return nil
	}

	Add(task)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err := Shutdown(ctx)
	if err != nil {
		t.Fatalf("Shutdown #1 error: %v", err)
	}

	if got := count.Load(); got != 1 {
		t.Fatalf("expected count=1 after first shutdown; got %d", got)
	}

	err = Shutdown(ctx)
	if err != nil {
		t.Fatalf("Shutdown #2 expected nil; got %v", err)
	}

	if got := count.Load(); got != 1 {
		t.Fatalf("expected count to remain 1; got %d", got)
	}
}

//nolint:paralleltest
func TestAddAfterShutdownReturnsClosed(t *testing.T) {
	resetQueue(t)

	started := make(chan struct{})
	unblock := make(chan struct{})
	blocker := func(ctx context.Context) error {
		close(started)
		<-unblock

		return nil
	}

	// Register a no-op then blocker; LIFO: blocker, noop.
	Add(func(ctx context.Context) error { return nil })
	Add(blocker)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})

	go func() {
		_ = Shutdown(ctx)

		close(done)
	}()

	<-started

	// New behavior: Add during/after shutdown is a no-op, not an error.
	var ran bool
	Add(func(ctx context.Context) error {
		ran = true
		return nil
	})

	close(unblock)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("Shutdown did not finish")
	}

	// Ensure the task added after shutdown start did not run.
	if ran {
		t.Fatalf("task added after shutdown should not run")
	}
}

//nolint:paralleltest
func TestTaskErrorsAreJoinedAndDetectable(t *testing.T) {
	resetQueue(t)

	err1 := errors.New("alpha")
	err2 := errors.New("beta")

	Add(func(ctx context.Context) error { return err1 })
	Add(func(ctx context.Context) error { return err2 })

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	shErr := Shutdown(ctx)
	if shErr == nil {
		t.Fatalf("expected joined error; got nil")
	}

	if !errors.Is(shErr, err1) || !errors.Is(shErr, err2) {
		t.Fatalf("expected joined error to contain both; got: %v", shErr)
	}

	s := shErr.Error()
	if !strings.Contains(s, "alpha") || !strings.Contains(s, "beta") {
		t.Fatalf("expected combined error string to include both messages; got: %q", s)
	}
}

//nolint:paralleltest
func TestShutdownWithNoTasksIsNil(t *testing.T) {
	resetQueue(t)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err := Shutdown(ctx)
	if err != nil {
		t.Fatalf("expected nil when no tasks; got %v", err)
	}

	err = Shutdown(ctx)
	if err != nil {
		t.Fatalf("expected nil on repeated shutdown with no tasks; got %v", err)
	}
}
