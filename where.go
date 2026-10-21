package crud

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// ErrEmptyWhere is returned when a Where has no conditions. Update, Read and
// Delete never touch a whole table by accident; use Get to read everything.
var ErrEmptyWhere = errors.New("crud-depot: where needs at least one condition")

// Where is a list of column = value conditions joined with AND, in order.
// Build it with Eq:
//
//	crud.Eq("organization_id", orgID).And("id", unitID)
//
// A nil value matches NULL (column IS NULL).
type Where []Cond

// Cond is one condition of a Where.
type Cond struct {
	Column string
	Value  any
}

// Eq starts a Where with column = value.
func Eq(column string, value any) Where {
	return Where{{Column: column, Value: value}}
}

// And adds column = value.
func (w Where) And(column string, value any) Where {
	out := make(Where, len(w), len(w)+1)
	copy(out, w)
	return append(out, Cond{Column: column, Value: value})
}

var identifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// clause renders the conditions; placeholders continue after the n
// arguments the query already has.
func (w Where) clause(d Dialect, n int) (string, []any, error) {
	if len(w) == 0 {
		return "", nil, ErrEmptyWhere
	}
	parts := make([]string, 0, len(w))
	args := make([]any, 0, len(w))
	for _, c := range w {
		// Column names go into the SQL text, so only plain identifiers.
		if !identifier.MatchString(c.Column) {
			return "", nil, fmt.Errorf("crud-depot: invalid column name %q in where", c.Column)
		}
		if c.Value == nil {
			parts = append(parts, c.Column+" IS NULL")
			continue
		}
		args = append(args, c.Value)
		parts = append(parts, fmt.Sprintf("%s = %s", c.Column, d.Placeholder(n+len(args))))
	}
	return strings.Join(parts, " AND "), args, nil
}

// ContextExecutor is implemented by *sql.DB, *sql.Tx and *sql.Conn. When
// the Executor given to New (or WithTx) also implements it, the ...Context
// methods pass their context down to the driver.
type ContextExecutor interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// TxContextBeginner is implemented by *sql.DB - used by RunInTxContext.
type TxContextBeginner interface {
	BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
}

// noContext adapts an Executor without context support; the context is
// dropped.
type noContext struct{ ex Executor }

func (n noContext) QueryContext(_ context.Context, q string, args ...any) (*sql.Rows, error) {
	return n.ex.Query(q, args...)
}

func (n noContext) QueryRowContext(_ context.Context, q string, args ...any) *sql.Row {
	return n.ex.QueryRow(q, args...)
}

func (n noContext) ExecContext(_ context.Context, q string, args ...any) (sql.Result, error) {
	return n.ex.Exec(q, args...)
}

func (c *CRUD) ctxDB() ContextExecutor {
	if ce, ok := c.db.(ContextExecutor); ok {
		return ce
	}
	return noContext{c.db}
}
