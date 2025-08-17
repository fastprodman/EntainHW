// Package shutdownqueue provides a process-wide, init()-initialized
// LIFO shutdown queue for cleanup tasks.
//
// Register tasks anywhere (including in your own init() funcs) via Add,
// and drain them explicitly at the end of main with:
//
//	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
//	defer cancel()
//	defer shutdownqueue.Shutdown(ctx) // or linter-friendly wrapper
//
// Tasks run once, in reverse order of registration. Panics are recovered.
// Shutdown is idempotent and returns an aggregated error via errors.Join.
package shutdownqueue

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// Task is a shutdown function. It should honor ctx and return an error
// if it can't finish (or ctx is canceled).
type Task func(ctx context.Context) error

type queue struct {
	mu     sync.Mutex
	tasks  []Task
	closed bool
}

var (
	q         *queue
	onceSetup sync.Once
)

func init() {
	onceSetup.Do(func() {
		q = &queue{tasks: make([]Task, 0, 8)}
	})
}

// Add registers a task to be run on Shutdown, in LIFO order.
// Safe to call from any goroutine, including in init().
// If t is nil or shutdown has already started, Add does nothing.
func Add(t Task) {
	if t == nil {
		return
	}

	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return
	}

	q.tasks = append(q.tasks, t)
}

// Shutdown drains all registered tasks in LIFO order.
// It is safe to call multiple times; after the first complete (or partial) run,
// subsequent calls are no-ops.
//
// If ctx is canceled or times out mid-drain, Shutdown stops early and returns
// an error that includes both the context error and any task errors so far,
// joined with errors.Join.
func Shutdown(ctx context.Context) error {
	// Atomically take ownership of tasks and mark closed.
	q.mu.Lock()

	if q.closed && len(q.tasks) == 0 {
		q.mu.Unlock()

		return nil
	}

	q.closed = true

	tasks := q.tasks

	q.tasks = nil

	q.mu.Unlock()

	var errs []error

	// Run in strict LIFO.
	for i := len(tasks) - 1; i >= 0; i-- {
		// Respect cancellation/timeout.
		select {
		case <-ctx.Done():
			errs = append(errs, fmt.Errorf("shutdown canceled: %w", ctx.Err()))

			return errors.Join(errs...)
		default:
		}

		// Run task with panic safety.
		func(t Task) {
			defer func() {
				r := recover()
				if r != nil {
					errs = append(errs, fmt.Errorf("panic in shutdown task: %v", r))
				}
			}()

			err := t(ctx)
			if err != nil {
				errs = append(errs, err)
			}
		}(tasks[i])
	}

	return errors.Join(errs...)
}
