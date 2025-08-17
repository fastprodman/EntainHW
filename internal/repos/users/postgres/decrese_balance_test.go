package users

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/fastprodman/EntainHW/internal/infra/pgtestutil"
	"github.com/fastprodman/EntainHW/internal/repos/users"
)

func TestUsers_DecreaseBalance_Table(t *testing.T) {
	t.Parallel()

	type seedFn func(db *sql.DB, t *testing.T)
	type tc struct {
		name          string
		seed          seedFn
		userID        uint64
		amount        int64
		wantBalance   int64
		wantErr       bool // true -> expect usersdom.ErrInsufficientFunds
		checkFinalBal bool // whether to check final balance (skip if user doesn't exist)
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
			name:          "sufficient_funds_decrease_from_positive",
			seed:          func(db *sql.DB, t *testing.T) { upsert(db, 201, 1_000, t) },
			userID:        201,
			amount:        250,
			wantBalance:   750,
			wantErr:       false,
			checkFinalBal: true,
		},
		{
			name:          "sufficient_funds_exact_to_zero",
			seed:          func(db *sql.DB, t *testing.T) { upsert(db, 202, 300, t) },
			userID:        202,
			amount:        300,
			wantBalance:   0,
			wantErr:       false,
			checkFinalBal: true,
		},
		{
			name:          "insufficient_funds_balance_unchanged",
			seed:          func(db *sql.DB, t *testing.T) { upsert(db, 203, 200, t) },
			userID:        203,
			amount:        300,
			wantBalance:   200, // should remain unchanged
			wantErr:       true,
			checkFinalBal: true,
		},
		{
			name:          "user_missing_treated_as_insufficient",
			seed:          func(_ *sql.DB, _ *testing.T) {}, // no user
			userID:        999_999,
			amount:        100,
			wantBalance:   0,     // not checked
			wantErr:       true,  // DecreaseBalance returns ErrInsufficientFunds when 0 rows affected
			checkFinalBal: false, // user doesn't exist -> skip balance check
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

			err = repo.DecreaseBalance(tx, tt.userID, tt.amount)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error (insufficient or missing), got nil")
				}
				if !errors.Is(err, users.ErrInsufficientFunds) {
					t.Fatalf("expected ErrInsufficientFunds, got: %v", err)
				}
				// no commit on error
			} else {
				if err != nil {
					t.Fatalf("decrease balance: %v", err)
				}
				err = tx.Commit()
				if err != nil {
					t.Fatalf("commit: %v", err)
				}
			}

			if tt.checkFinalBal {
				got, gerr := repo.GetBalance(ctx, tt.userID)
				if gerr != nil {
					t.Fatalf("get balance after decrease: %v", gerr)
				}
				if got != tt.wantBalance {
					t.Fatalf("final balance mismatch: want %d, got %d", tt.wantBalance, got)
				}
			}
		})
	}
}

func TestUsers_DecreaseBalance_ConcurrentGuard(t *testing.T) {
	t.Parallel()

	db, cleanup := pgtestutil.NewTestDB(t)
	defer cleanup()

	repo := New(db)

	// Seed one user with balance = 1000
	_, err := db.Exec(`INSERT INTO users (id, balance) VALUES ($1, $2)`, 1, 1000)
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	success, insufficient := 0, 0

	worker := func(name string) {
		defer wg.Done()

		ctx := context.Background()
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Errorf("[%s] begin tx: %v", name, err)
			return
		}
		defer tx.Rollback()

		// Lock row first (this will serialize)
		_, err = repo.LockAndGetBalance(tx, 1)
		if err != nil {
			t.Errorf("[%s] lock balance: %v", name, err)
			return
		}

		// Try to decrease 1000
		err = repo.DecreaseBalance(tx, 1, 1000)
		if err == nil {
			mu.Lock()
			success++
			mu.Unlock()
			if err := tx.Commit(); err != nil {
				t.Errorf("[%s] commit: %v", name, err)
			}
			return
		}

		if errors.Is(err, users.ErrInsufficientFunds) {
			mu.Lock()
			insufficient++
			mu.Unlock()
			_ = tx.Rollback()
			return
		}

		t.Errorf("[%s] unexpected error: %v", name, err)
	}

	wg.Add(2)
	go worker("A")
	go worker("B")
	wg.Wait()

	if success != 1 || insufficient != 1 {
		t.Fatalf("want 1 success and 1 insufficient, got success=%d insufficient=%d", success, insufficient)
	}
}
