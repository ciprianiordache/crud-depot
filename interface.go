package crud

import (
	"database/sql"
	"time"
)

// Executor is implemented by *sql.DB and *sql.Tx - allows
// the same CRUD instance to work inside or outside a transaction.
type Executor interface {
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
	Exec(query string, args ...any) (sql.Result, error)
}

// TxBeginner is implemented by *sql.DB and allows crud-depot
// to begin a transaction internally via RunInTx.
type TxBeginner interface {
	Executor
	Begin() (*sql.Tx, error)
}

type Dialect interface {
	Placeholder(n int) string
	ReturningClause(col string) string // Postgres: "RETURNING id" | MySQL/SQLite: ""
	UsesLastInsertID() bool            // MySQL/SQLite: true | Postgres: false
}

// TableNamer — model provide its own table name.
//
// func (User) TableName() string { return "users" }
type TableNamer interface {
	TableName() string
}

// Timestamped — model wants automatic created_at / updated_at management.
// crud-depot will call these before Create and Update
//
// func (u *User) SetCreatedAt(t time.Time) { u.CreatedAt = t }
// func (u *User) SetUpdatedAt(t time.Time) { u.UpdatedAt = t }
type Timestamped interface {
	SetCreatedAt(t time.Time)
	SetUpdatedAt(t time.Time)
}

// SoftDeleter — model wants soft delete instead of physical DELETE.
// crud-depot will issue UPDATE <table> SET <fuekd> = NOW() instead of DELETE.
//
// func (User) SoftDeleteField() stirng { return "deleted_at" }
type SoftDeleter interface {
	SoftDeleteField() string
}
