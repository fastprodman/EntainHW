package users

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/fastprodman/EntainHW/internal/infra/pgtestutil"
)

func TestUsers_LockAndGetBalance_Table(t *testing.T) {
	t.Parallel()

	type seedFn func(db *sql.DB, t *testing.T)
	type tc struct {
		name        string
		seed        seedFn
		userID      uint64
		wantBalance int64
		wantErr     bool // true => expect error (e.g., user not found)
	}

	tests := []tc{
		{
			name: "user_exists_zero_balance",
			seed: func(db *sql.DB, t *testing.T) {
				_, err := db.Exec(`INSERT INTO users (id, balance) VALUES ($1, $2)`, 1, 0)
				if err != nil {
					t.Fatalf("seed user: %v", err)
				}
			},
			userID:      1,
			wantBalance: 0,
			wantErr:     false,
		},
		{
			name: "user_exists_positive_balance",
			seed: func(db *sql.DB, t *testing.T) {
				_, err := db.Exec(`INSERT INTO users (id, balance) VALUES ($1, $2)`, 2, 12345)
				if err != nil {
					t.Fatalf("seed user: %v", err)
				}
			},
			userID:      2,
			wantBalance: 12345,
			wantErr:     false,
		},
		{
			name:        "user_not_found",
			seed:        func(_ *sql.DB, _ *testing.T) {},
			userID:      999,
			wantBalance: 0,
			wantErr:     true, // expect wrapped sql.ErrNoRows
		},
		{
			name: "user_exists_large_balance",
			seed: func(db *sql.DB, t *testing.T) {
				// within BIGINT but large enough to matter (e.g., 9e14 cents = 9T cents)
				_, err := db.Exec(`INSERT INTO users (id, balance) VALUES ($1, $2)`, 3, int64(900_000_000_000_000))
				if err != nil {
					t.Fatalf("seed user: %v", err)
				}
			},
			userID:      3,
			wantBalance: int64(900_000_000_000_000),
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// t.Parallel()

			db, cleanup := pgtestutil.NewTestDB(t)
			defer cleanup()

			var cnt int
			_ = db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&cnt)
			t.Logf("users count after base migrations: %d", cnt)
			fmt.Printf("users count after base migrations: %d\n", cnt)

			if tt.seed != nil {
				tt.seed(db, t)
			}

			repo := New(db)

			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()

			tx, err := db.BeginTx(ctx, nil)
			if err != nil {
				t.Fatalf("begin tx: %v", err)
			}
			defer func() { _ = tx.Rollback() }()

			bal, err := repo.LockAndGetBalance(tx, tt.userID)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (balance=%d)", bal)
				}
				// Expect the wrapped error to contain sql.ErrNoRows
				if !errors.Is(err, sql.ErrNoRows) {
					t.Fatalf("expected sql.ErrNoRows, got: %v", err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if bal != tt.wantBalance {
				t.Fatalf("balance mismatch: want %d, got %d", tt.wantBalance, bal)
			}

			err = tx.Commit()
			if err != nil {
				t.Fatalf("commit: %v", err)
			}
		})
	}
}

// Concurrency/locking behavior: second FOR UPDATE on same row should block until first tx commits.
func TestUsers_LockAndGetBalance_LocksRow(t *testing.T) {
	t.Parallel()

	db, cleanup := pgtestutil.NewTestDB(t)
	defer cleanup()

	_, err := db.Exec(`INSERT INTO users (id, balance) VALUES ($1, $2)`, 42, 200)
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}

	repo := New(db)

	// tx1 locks the row
	ctx1, cancel1 := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel1()

	tx1, err := db.BeginTx(ctx1, nil)
	if err != nil {
		t.Fatalf("begin tx1: %v", err)
	}
	defer func() { _ = tx1.Rollback() }()

	_, err = repo.LockAndGetBalance(tx1, 42)
	if err != nil {
		t.Fatalf("tx1 lock/get: %v", err)
	}

	// Now start tx2 which should block trying to lock the same row
	blockedCh := make(chan struct{})
	doneCh := make(chan struct{})
	errCh := make(chan error, 1)

	go func() {
		defer close(doneCh)

		ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel2()

		tx2, e := db.BeginTx(ctx2, nil)
		if e != nil {
			errCh <- e
			return
		}
		defer func() { _ = tx2.Rollback() }()

		// Signal that we started and will likely block on FOR UPDATE
		close(blockedCh)

		_, e = repo.LockAndGetBalance(tx2, 42)
		if e != nil {
			errCh <- e
			return
		}

		e = tx2.Commit()
		if e != nil {
			errCh <- e
			return
		}
	}()

	// Wait until goroutine is trying to lock
	select {
	case <-blockedCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for tx2 to start")
	}

	// Give it a moment to attempt the lock (and block)
	time.Sleep(200 * time.Millisecond)

	// Commit tx1 to release the lock so tx2 can proceed
	err = tx1.Commit()
	if err != nil {
		t.Fatalf("commit tx1: %v", err)
	}

	// Now tx2 should finish without error
	select {
	case e := <-errCh:
		if e != nil {
			t.Fatalf("tx2 error: %v", e)
		}
	case <-doneCh:
		// done without pushing an error (OK)
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for tx2 to complete after tx1 commit")
	}
}
