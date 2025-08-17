package users

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/fastprodman/EntainHW/internal/infra/pgtestutil"
)

func TestUsers_IncreaseBalance_Basic(t *testing.T) {
	t.Parallel()

	type seedFn func(db *sql.DB, t *testing.T)
	type tc struct {
		name        string
		seed        seedFn
		userID      uint64
		amount      int64
		wantBalance int64
	}

	upsert := func(db *sql.DB, id uint64, bal int64, t *testing.T) {
		_, err := db.Exec(`
			INSERT INTO users (id, balance) VALUES ($1, $2)
			ON CONFLICT (id) DO UPDATE SET balance = EXCLUDED.balance
		`, id, bal)
		if err != nil {
			t.Fatalf("seed upsert user(%d): %v", id, err)
		}
	}

	tests := []tc{
		{
			name:        "increase_from_zero",
			seed:        func(db *sql.DB, t *testing.T) { upsert(db, 101, 0, t) },
			userID:      101,
			amount:      250, // +2.50
			wantBalance: 250,
		},
		{
			name:        "increase_from_positive",
			seed:        func(db *sql.DB, t *testing.T) { upsert(db, 102, 1_000, t) },
			userID:      102,
			amount:      500, // +5.00
			wantBalance: 1_500,
		},
		{
			name:        "increase_large_balance",
			seed:        func(db *sql.DB, t *testing.T) { upsert(db, 103, 900_000_000_000_000, t) },
			userID:      103,
			amount:      123,
			wantBalance: 900_000_000_000_123,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			db, cleanup := pgtestutil.NewTestDB(t)
			defer cleanup()

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

			err = repo.IncreaseBalance(tx, tt.userID, tt.amount)
			if err != nil {
				t.Fatalf("increase balance: %v", err)
			}

			err = tx.Commit()
			if err != nil {
				t.Fatalf("commit: %v", err)
			}

			got, err := repo.GetBalance(ctx, tt.userID)
			if err != nil {
				t.Fatalf("get balance: %v", err)
			}
			if got != tt.wantBalance {
				t.Fatalf("balance mismatch: want %d, got %d", tt.wantBalance, got)
			}
		})
	}
}

func TestUsers_IncreaseBalance_ConcurrentAdds(t *testing.T) {
	t.Parallel()

	db, cleanup := pgtestutil.NewTestDB(t)
	defer cleanup()

	// seed user with 0 balance
	_, err := db.Exec(`INSERT INTO users (id, balance) VALUES ($1, $2) ON CONFLICT (id) DO NOTHING`, 777, 0)
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}

	repo := New(db)

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	errCh := make(chan error, 2)
	doneCh := make(chan struct{}, 2)

	worker := func(amount int64) {
		defer func() { doneCh <- struct{}{} }()

		tx, e := db.BeginTx(ctx, nil)
		if e != nil {
			errCh <- e
			return
		}
		defer func() { _ = tx.Rollback() }()

		e = repo.IncreaseBalance(tx, 777, amount)
		if e != nil {
			errCh <- e
			return
		}
		e = tx.Commit()
		if e != nil {
			errCh <- e
			return
		}
	}

	// Two concurrent increments
	go worker(1_000) // +10.00
	go worker(2_500) // +25.00

	for i := 0; i < 2; i++ {
		select {
		case e := <-errCh:
			if e != nil {
				t.Fatalf("worker error: %v", e)
			}
		case <-doneCh:
			// ok
		case <-ctx.Done():
			t.Fatalf("timeout waiting for workers")
		}
	}

	// Verify final balance is the sum
	got, err := repo.GetBalance(ctx, 777)
	if err != nil {
		t.Fatalf("get balance: %v", err)
	}
	want := int64(3_500)
	if got != want {
		t.Fatalf("final balance mismatch: want %d, got %d", want, got)
	}
}

// Optional: document current behavior for non-existent user (method returns nil).
func TestUsers_IncreaseBalance_UserNotFound_CurrentBehavior(t *testing.T) {
	t.Parallel()

	db, cleanup := pgtestutil.NewTestDB(t)
	defer cleanup()

	repo := New(db)

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Call IncreaseBalance for a user that doesn't exist.
	err = repo.IncreaseBalance(tx, 999_999, 100)
	if err != nil {
		t.Fatalf("increase balance unexpected error: %v", err)
	}

	err = tx.Commit()
	if err != nil {
		t.Fatalf("commit: %v", err)
	}

	// Confirm the user still doesn't exist.
	_, err = repo.GetBalance(ctx, 999_999)
	if err == nil {
		t.Fatalf("expected error for missing user, got nil")
	}
	// Usually your repo maps to domain error; if not, expect sql.ErrNoRows.
	if !errors.Is(err, sql.ErrNoRows) {
		// If you have a domain error like users.ErrUserNotFound, replace sql.ErrNoRows above.
		t.Logf("note: GetBalance returned a non-sql.ErrNoRows error: %v", err)
	}
}
