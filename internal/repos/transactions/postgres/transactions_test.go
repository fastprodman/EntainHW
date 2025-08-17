package transactions

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/fastprodman/EntainHW/internal/infra/pgtestutil"
	"github.com/fastprodman/EntainHW/internal/repos/transactions"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestTransactions_Insert(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		seed    func(db *sql.DB) // prepare users/transactions if needed
		txid    string
		userID  uint64
		wantErr error
	}{
		{
			name: "ok_insert",
			seed: func(db *sql.DB) {
				_, err := db.Exec(`INSERT INTO users (id, balance) VALUES ($1, $2)`, 1, 100)
				if err != nil {
					t.Fatalf("seed user: %v", err)
				}
			},
			txid:    "tx_123",
			userID:  1,
			wantErr: nil,
		},
		{
			name: "duplicate_transaction",
			seed: func(db *sql.DB) {
				_, err := db.Exec(`INSERT INTO users (id, balance) VALUES ($1, $2)`, 2, 100)
				if err != nil {
					t.Fatalf("seed user: %v", err)
				}
				_, err = db.Exec(`INSERT INTO transactions (transaction_id, user_id) VALUES ($1, $2)`, "tx_dup", 2)
				if err != nil {
					t.Fatalf("seed tx: %v", err)
				}
			},
			txid:    "tx_dup",
			userID:  2,
			wantErr: transactions.ErrDuplicateTransaction,
		},
		{
			name:    "user_not_exist_fk_violation",
			seed:    func(db *sql.DB) {}, // no user seeded
			txid:    "tx_fk",
			userID:  999,
			wantErr: &pgconn.PgError{}, // expect a wrapped pg error
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			db, cleanup := pgtestutil.NewTestDB(t)
			defer cleanup()

			repo := New(db)

			if tt.seed != nil {
				tt.seed(db)
			}

			ctx := context.Background()
			tx, err := db.BeginTx(ctx, nil)
			if err != nil {
				t.Fatalf("begin tx: %v", err)
			}
			defer tx.Rollback()

			err = repo.Insert(tx, tt.txid, tt.userID)

			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}

			// Handle pg error type separately
			var pgErr *pgconn.PgError
			if errors.As(tt.wantErr, &pgErr) {
				if !errors.As(err, &pgErr) {
					t.Fatalf("expected pg error, got %v", err)
				}
				return
			}

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("unexpected error: got %v, want %v", err, tt.wantErr)
			}
		})
	}
}
