package users

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/fastprodman/EntainHW/internal/infra/pgtestutil"
	"github.com/fastprodman/EntainHW/internal/repos/users"
)

func TestUsers_Exists_TableDriven(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		seed    func(db *sql.DB) // optional seeding
		userID  uint64
		wantErr error
	}{
		{
			name: "user exists",
			seed: func(db *sql.DB) {
				_, err := db.Exec(`INSERT INTO users (id, balance) VALUES ($1, $2)`, 42, 100)
				if err != nil {
					t.Fatalf("seed user: %v", err)
				}
			},
			userID:  42,
			wantErr: nil,
		},
		{
			name:    "user not found",
			seed:    func(db *sql.DB) {}, // no user
			userID:  999,
			wantErr: users.ErrUserNotFound,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			db, cleanup := pgtestutil.NewTestDB(t)
			defer cleanup()

			repo := New(db)

			// seed if needed
			if tt.seed != nil {
				tt.seed(db)
			}

			ctx := context.Background()
			tx, err := db.BeginTx(ctx, nil)
			if err != nil {
				t.Fatalf("begin tx: %v", err)
			}
			defer tx.Rollback()

			err = repo.Exists(tx, tt.userID)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("unexpected error: got %v, want %v", err, tt.wantErr)
			}
		})
	}
}
