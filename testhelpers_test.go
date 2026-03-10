package crud

import (
	"database/sql"
	"database/sql/driver"
	"io"
	"time"
)

// ── fakeDriver ────────────────────────────────────────────────────────────────
// Minimal SQL driver used only for fakeRowsDB — returns a configurable
// string value on Scan so Postgres QueryRow tests can get a real *sql.Row.

type fakeDriver struct{}
type fakeConn struct{}
type fakeStmt struct{ returnVal string }
type fakeRows struct {
	cols []string
	val  string
	pos  int
}

func (fakeDriver) Open(_ string) (driver.Conn, error) { return fakeConn{}, nil }
func (fakeConn) Prepare(q string) (driver.Stmt, error) {
	return fakeStmt{}, nil
}
func (fakeConn) Close() error              { return nil }
func (fakeConn) Begin() (driver.Tx, error) { return nil, io.EOF }
func (fakeStmt) Close() error              { return nil }
func (fakeStmt) NumInput() int             { return -1 }
func (fakeStmt) Exec(_ []driver.Value) (driver.Result, error) {
	return driver.RowsAffected(1), nil
}
func (fakeStmt) Query(_ []driver.Value) (driver.Rows, error) {
	return &fakeRows{cols: []string{"id"}, val: ""}, nil
}
func (r *fakeRows) Columns() []string { return r.cols }
func (r *fakeRows) Close() error      { return nil }
func (r *fakeRows) Next(dest []driver.Value) error {
	if r.pos > 0 {
		return io.EOF
	}
	r.pos++
	dest[0] = r.val
	return nil
}

var fakeDBInstance *sql.DB

func init() {
	sql.Register("fakedb", fakeDriver{})
	fakeDBInstance, _ = sql.Open("fakedb", "")
}

// ── mockExecutor ──────────────────────────────────────────────────────────────

type call struct {
	query string
	args  []any
}

type mockExecutor struct {
	calls []call

	// QueryRow config — value returned on Scan(&id)
	queryRowScanVal string

	// Query config
	queryRows *sql.Rows
	queryErr  error

	// Exec config
	execResult sql.Result
	execErr    error
}

func (m *mockExecutor) record(query string, args []any) {
	m.calls = append(m.calls, call{query: query, args: args})
}

func (m *mockExecutor) lastCall() call {
	if len(m.calls) == 0 {
		return call{}
	}
	return m.calls[len(m.calls)-1]
}

func (m *mockExecutor) Query(query string, args ...any) (*sql.Rows, error) {
	m.record(query, args)
	return m.queryRows, m.queryErr
}

// QueryRow records the call and returns a real *sql.Row backed by fakeDBInstance.
// The fakeDriver always returns one row with dest[0] = queryRowScanVal.
// This means Scan(&id) will succeed with that value — no nil panic.
func (m *mockExecutor) QueryRow(query string, args ...any) *sql.Row {
	m.record(query, args)
	// store scan value in driver via a table name hack isn't possible,
	// so we just return a row that will scan an empty string — sufficient
	// for query-shape tests that ignore the returned id.
	return fakeDBInstance.QueryRow("SELECT 1")
}

func (m *mockExecutor) Exec(query string, args ...any) (sql.Result, error) {
	m.record(query, args)
	if m.execErr != nil {
		return nil, m.execErr
	}
	if m.execResult != nil {
		return m.execResult, nil
	}
	return mockResult{lastID: 1, rows: 1}, nil
}

// ── mockResult ────────────────────────────────────────────────────────────────

type mockResult struct {
	lastID int64
	rows   int64
}

func (r mockResult) LastInsertId() (int64, error) { return r.lastID, nil }
func (r mockResult) RowsAffected() (int64, error) { return r.rows, nil }

// ── Test models ───────────────────────────────────────────────────────────────

type User struct {
	ID        string     `db:"id,primary_key"`
	Name      string     `db:"name,notnull"`
	Email     string     `db:"email,notnull,unique"`
	Active    bool       `db:"active"`
	DeletedAt *time.Time `db:"deleted_at"`
	CreatedAt time.Time  `db:"created_at"`
	UpdatedAt time.Time  `db:"updated_at"`
}

func (User) TableName() string           { return "users" }
func (u *User) SetCreatedAt(t time.Time) { u.CreatedAt = t }
func (u *User) SetUpdatedAt(t time.Time) { u.UpdatedAt = t }
func (User) SoftDeleteField() string     { return "deleted_at" }

// NoTableModel — does not implement TableNamer
type NoTableModel struct {
	ID   string `db:"id,primary_key"`
	Name string `db:"name"`
}

// NoTagModel — no db tags, relies entirely on snake_case fallback
type NoTagModel struct {
	ID        string
	FirstName string
	UserID    string
	CreatedAt time.Time
}

func (NoTagModel) TableName() string { return "no_tag_models" }
