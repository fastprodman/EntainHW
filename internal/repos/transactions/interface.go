package transactions

import (
	"database/sql"
	"errors"
)

var ErrDuplicateTransaction = errors.New("duplicate transaction")

type Transactions interface {
	Insert(tx *sql.Tx, txid string, userID uint64) error
}
