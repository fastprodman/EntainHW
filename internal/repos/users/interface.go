package users

import (
	"context"
	"database/sql"
	"errors"
)

var ErrInsufficientFunds = errors.New("insufficient funds")
var ErrUserNotFound = errors.New("user not found")

type Users interface {
	Exists(tx *sql.Tx, userID uint64) error
	GetBalance(ctx context.Context, userID uint64) (int64, error)
	LockAndGetBalance(tx *sql.Tx, userID uint64) (int64, error)
	IncreaseBalance(tx *sql.Tx, userID uint64, amount int64) error
	DecreaseBalance(tx *sql.Tx, userID uint64, amount int64) error
}
