package users

import (
	"database/sql"
	"testing"

	"github.com/fastprodman/EntainHW/internal/infra/pgtestutil"
)

func TestUsers_GetBalance_TableDriven(t *testing.T) {
	t.Parallel() // allow this suite to run alongside others

	type tc struct {
		name        string
		seed        func(db *sql.DB, t *testing.T)
		userID      uint64
		wantBalance int64
		wantErr     bool
	}

	tests := []tc{
		{
			name: "ok_user_exists",
			seed: func(db *sql.DB, t *testing.T) {
				_, err := db.Exec(`INSERT INTO users (id, balance) VALUES (1, 1000)`)
				if err != nil {
					t.Fatalf("seed user: %v", err)
				}
			},
			userID:      1,
			wantBalance: 1000,
			wantErr:     false,
		},
		{
			name:        "error_user_not_found",
			seed:        nil, // no seed -> user missing
			userID:      999,
			wantBalance: 0,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		tt := tt // pin
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			db, cleanup := pgtestutil.NewTestDB(t)
			defer cleanup()

			if tt.seed != nil {
				tt.seed(db, t)
			}

			repo := New(db)

			ctx := t.Context() // use test-scoped context (cancels on test end)

			gotBalance, err := repo.GetBalance(ctx, tt.userID)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got nil (balance=%d)", gotBalance)
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if gotBalance != tt.wantBalance {
				t.Fatalf("balance: want %d, got %d", tt.wantBalance, gotBalance)
			}
		})
	}
}
