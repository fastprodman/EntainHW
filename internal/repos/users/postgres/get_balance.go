package users

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/fastprodman/EntainHW/internal/repos/users"
)

func (r *usersRepo) GetBalance(ctx context.Context, userID uint64) (int64, error) {
	var balance int64

	err := r.db.QueryRowContext(ctx, `
		SELECT balance
		FROM users
		WHERE id = $1
	`, userID).Scan(&balance)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, users.ErrUserNotFound
		}

		return 0, fmt.Errorf("get balance: %w", err)
	}

	return balance, nil
}
