package crud

import (
	"database/sql"
	"database/sql/driver"
	"io"
	"testing"
	"time"
)

// nullDriver returns two fixed rows - one with values, one with NULLs - so
// scanRows can be checked against pointer and sql.Null* fields.
type nullDriver struct{}
type nullConn struct{}
type nullStmt struct{}
type nullRows struct{ pos int }

var nullCols = []string{"name", "nick", "age", "seen_at", "score"}

func (nullDriver) Open(string) (driver.Conn, error)         { return nullConn{}, nil }
func (nullConn) Prepare(string) (driver.Stmt, error)        { return nullStmt{}, nil }
func (nullConn) Close() error                               { return nil }
func (nullConn) Begin() (driver.Tx, error)                  { return nil, io.EOF }
func (nullStmt) Close() error                               { return nil }
func (nullStmt) NumInput() int                              { return -1 }
func (nullStmt) Exec([]driver.Value) (driver.Result, error) { return driver.RowsAffected(0), nil }
func (nullStmt) Query([]driver.Value) (driver.Rows, error)  { return &nullRows{}, nil }
func (*nullRows) Columns() []string                         { return nullCols }
func (*nullRows) Close() error                              { return nil }

var seenAt = time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)

func (r *nullRows) Next(dest []driver.Value) error {
	switch r.pos {
	case 0:
		copy(dest, []driver.Value{"Ana", "ani", int64(31), seenAt, int64(7)})
	case 1:
		copy(dest, []driver.Value{"Bogdan", nil, nil, nil, nil})
	default:
		return io.EOF
	}
	r.pos++
	return nil
}

func init() { sql.Register("crud_nullfake", nullDriver{}) }

type person struct {
	Name   string        `db:"name"`
	Nick   *string       `db:"nick"`
	Age    *int64        `db:"age"`
	SeenAt *time.Time    `db:"seen_at"`
	Score  sql.NullInt64 `db:"score"`
}

func (person) TableName() string { return "people" }

func TestScanRows_PointerAndScannerFields(t *testing.T) {
	db, err := sql.Open("crud_nullfake", "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var got []person
	if err := New(db, Postgres{}).Get(person{}, &got, 0, 10); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d rows, want 2", len(got))
	}

	a := got[0]
	if a.Name != "Ana" || a.Nick == nil || *a.Nick != "ani" || a.Age == nil || *a.Age != 31 ||
		a.SeenAt == nil || !a.SeenAt.Equal(seenAt) || !a.Score.Valid || a.Score.Int64 != 7 {
		t.Fatalf("row with values scanned wrong: %+v", a)
	}

	b := got[1]
	if b.Name != "Bogdan" || b.Nick != nil || b.Age != nil || b.SeenAt != nil || b.Score.Valid {
		t.Fatalf("NULLs should leave pointers nil and Null* invalid: %+v", b)
	}
}
