package crud

import "fmt"

// Postgres
func (Postgres) Placeholder(n int) string {
	return fmt.Sprintf("$%d", n)
}

func (Postgres) ReturningClause(col string) string {
	return fmt.Sprintf("RETURNING %s", col)
}

func (Postgres) UsesLastInsertID() bool {
	return false
}

// MySQL
func (MySQL) Placeholder(_ int) string {
	return "?"
}

func (MySQL) ReturningClause(_ string) string {
	return ""
}

func (MySQL) UsesLastInsertID() bool {
	return true
}

// SQLite
func (SQLite) Placeholder(_ int) string {
	return "?"
}

func (SQLite) ReturningClause(_ string) string {
	return ""
}

func (SQLite) UsesLastInsertID() bool {
	return true
}
