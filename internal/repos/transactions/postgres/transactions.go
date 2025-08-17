package transactions

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/fastprodman/EntainHW/internal/repos/transactions"
	"github.com/jackc/pgx/v5/pgconn"
)

var _ transactions.Transactions = (*transactionsRepo)(nil)

type transactionsRepo struct{ db *sql.DB }

func New(db *sql.DB) *transactionsRepo {
	return &transactionsRepo{db: db}
}

func (r *transactionsRepo) Insert(tx *sql.Tx, txid string, userID uint64) error {
	_, err := tx.Exec(`
		INSERT INTO transactions (transaction_id, user_id)
		VALUES ($1, $2)
	`, txid, userID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if pgErr.Code == "23505" { // unique_violation
				return transactions.ErrDuplicateTransaction
			}
		}

		return fmt.Errorf("insert transaction: %w", err)
	}

	return nil
}
