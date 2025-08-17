package users

import (
	"database/sql"
	"fmt"
)

func (r *usersRepo) LockAndGetBalance(tx *sql.Tx, userID uint64) (int64, error) {
	var balance int64

	err := tx.QueryRow(`
		SELECT balance
		FROM users
		WHERE id = $1
		FOR UPDATE
	`, userID).Scan(&balance)
	if err != nil {
		return 0, fmt.Errorf("lock/get balance: %w", err)
	}

	return balance, nil
}
