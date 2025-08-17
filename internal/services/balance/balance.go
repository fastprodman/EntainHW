package balance

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/fastprodman/EntainHW/internal/infra/pgutils"
	"github.com/fastprodman/EntainHW/internal/repos/transactions"
	pgtransactions "github.com/fastprodman/EntainHW/internal/repos/transactions/postgres"
	"github.com/fastprodman/EntainHW/internal/repos/users"
	pgusers "github.com/fastprodman/EntainHW/internal/repos/users/postgres"
)

type BalanceService struct {
	db    *sql.DB
	users users.Users
	txns  transactions.Transactions
}

func New(dbx *sql.DB) *BalanceService {
	return &BalanceService{
		db:    dbx,
		users: pgusers.New(dbx),
		txns:  pgtransactions.New(dbx),
	}
}

// ProcessTransaction runs the full flow in a single DB transaction:
//
// 1) Ensure user exists.
// 2) Lock user row (FOR UPDATE).
// 3) Apply effect via repo calls.
// 4) Insert tx (unique-violation -> ErrDuplicateTransaction).
func (s *BalanceService) ProcessTransaction(ctx context.Context, transaction Transaction) error {
	err := pgutils.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		// 1) Ensure user exists
		err := s.users.Exists(tx, transaction.UserID)
		if err != nil {
			return fmt.Errorf("check user exists: %w", err)
		}

		// 2) Lock user row
		balance, err := s.users.LockAndGetBalance(tx, transaction.UserID)
		if err != nil {
			return fmt.Errorf("lock and get balance: %w", err)
		}

		// 3) Apply the effect
		switch transaction.State {
		case TxWin:
			err = s.users.IncreaseBalance(tx, transaction.UserID, transaction.AmountMinor)
			if err != nil {
				return fmt.Errorf("increase balance: %w", err)
			}

		case TxLose:
			// pre-check against locked balance
			if balance < transaction.AmountMinor {
				return fmt.Errorf("pre-check decrease: %w", users.ErrInsufficientFunds)
			}

			err = s.users.DecreaseBalance(tx, transaction.UserID, transaction.AmountMinor)
			if err != nil {
				return fmt.Errorf("decrease balance: %w", err)
			}

		default:
			return fmt.Errorf("invalid state: %s", transaction.State)
		}

		// 4) Insert transaction record
		err = s.txns.Insert(tx, transaction.TransactionID, transaction.UserID)
		if err != nil {
			return fmt.Errorf("insert transaction: %w", err)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("process transaction: %w", err)
	}

	return nil
}

// GetBalance returns the user's balance (no locks; suitable for the GET endpoint).
func (s *BalanceService) GetBalance(ctx context.Context, userID uint64) (int64, error) {
	balance, err := s.users.GetBalance(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("get balance: %w", err)
	}

	return balance, nil
}
