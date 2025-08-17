package users

import (
	"database/sql"
	"fmt"

	"github.com/fastprodman/EntainHW/internal/repos/users"
)

func (r *usersRepo) Exists(tx *sql.Tx, userID uint64) error {
	var exists bool

	err := tx.QueryRow(`
		SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)
	`, userID).Scan(&exists)
	if err != nil {
		return fmt.Errorf("check exists: %w", err)
	}

	if !exists {
		return users.ErrUserNotFound
	}

	return nil
}
