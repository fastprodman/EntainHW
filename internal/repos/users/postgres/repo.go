package users

import (
	"database/sql"
)

type usersRepo struct{ db *sql.DB }

func New(db *sql.DB) *usersRepo {
	return &usersRepo{db: db}
}
