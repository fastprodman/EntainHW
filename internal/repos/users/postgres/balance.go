package users

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/fastprodman/EntainHW/internal/repos/users"
)

type usersRepo struct{ db *sql.DB }

func New(db *sql.DB) *usersRepo {
	return &usersRepo{db: db}
}

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
