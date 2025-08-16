package balance

import "errors"

type SourceType string

const (
	SourceGame    SourceType = "game"
	SourceServer  SourceType = "server"
	SourcePayment SourceType = "payment"
)

type TxState string

const (
	TxWin  TxState = "win"
	TxLose TxState = "lose"
)

type Transaction struct {
	TransactionID string
	UserID        uint64
	Source        SourceType
	State         TxState
	AmountMinor   int64 // cents
}

type UserSnapshot struct {
	UserID       uint64
	BalanceMinor int64 // cents
}

var (
	ErrDuplicateTransaction = errors.New("duplicate transaction")
	ErrInsufficientFunds    = errors.New("insufficient funds")
)
