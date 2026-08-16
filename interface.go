package crud

import "database/sql"

// Executor is implemented by *sql.DB and *sql.Tx.
type Executor interface {
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
	Exec(query string, args ...any) (sql.Result, error)
}

// TxBeginner is implemented by *sql.DB — used by RunInTx.
type TxBeginner interface {
	Executor
	Begin() (*sql.Tx, error)
}

type Dialect interface {
	Placeholder(n int) string
	ReturningClause(col string) string
	UsesLastInsertID() bool
}

// TableNamer — model provides its own table name.
//
//	func (User) TableName() string { return "users" }
type TableNamer interface {
	TableName() string
}

// SoftDeleter — model uses soft delete instead of physical DELETE.
//
//	func (User) SoftDeleteField() string { return "deleted_at" }
type SoftDeleter interface {
	SoftDeleteField() string
}
