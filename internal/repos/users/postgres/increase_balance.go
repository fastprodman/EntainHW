package users

import (
	"database/sql"
	"fmt"
)

func (r *usersRepo) IncreaseBalance(tx *sql.Tx, userID uint64, amount int64) error {
	_, err := tx.Exec(`
		UPDATE users
		SET balance = balance + $2
		WHERE id = $1
	`, userID, amount)
	if err != nil {
		return fmt.Errorf("increase balance: %w", err)
	}

	return nil
}
