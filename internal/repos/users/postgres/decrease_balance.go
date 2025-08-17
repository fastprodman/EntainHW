package users

import (
	"database/sql"
	"fmt"

	"github.com/fastprodman/EntainHW/internal/repos/users"
)

func (r *usersRepo) DecreaseBalance(tx *sql.Tx, userID uint64, amount int64) error {
	res, err := tx.Exec(`
		UPDATE users
		SET balance = balance - $2
		WHERE id = $1
		  AND balance >= $2
	`, userID, amount)
	if err != nil {
		return fmt.Errorf("decrease balance: %w", err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}

	if affected == 0 {
		return users.ErrInsufficientFunds
	}

	return nil
}
