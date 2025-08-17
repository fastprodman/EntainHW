package pgutils

import (
	"context"
	"database/sql"
	"fmt"
)

// WithTx runs fn inside a transaction.
// It commits if fn returns nil, otherwise it rolls back.
func WithTx(ctx context.Context, db *sql.DB, fn func(*sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil) // default isolation level
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	err = fn(tx)
	if err != nil {
		rbErr := tx.Rollback()
		if rbErr != nil {
			return fmt.Errorf("rollback after fn error: %v (fn err: %w)", rbErr, err)
		}
		return fmt.Errorf("fn: %w", err)
	}

	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	return nil
}
