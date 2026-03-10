package crud

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func newPostgresCRUD(ex Executor) *CRUD {
	return &CRUD{db: ex, dialect: Postgres{}}
}

func newMySQLCRUD(ex Executor) *CRUD {
	return &CRUD{db: ex, dialect: MySQL{}}
}

// ── Create ────────────────────────────────────────────────────────────────────

func TestCreate_QueryShape_Postgres(t *testing.T) {
	// Use MySQL dialect so Create goes through Exec path (no QueryRow needed)
	ex := &mockExecutor{execResult: mockResult{lastID: 1}}
	c := newMySQLCRUD(ex)

	_, _ = c.Create(&User{Name: "Ion", Email: "ion@test.com"})

	q := ex.lastCall().query
	if !strings.HasPrefix(q, "INSERT INTO users") {
		t.Errorf("query should start with INSERT INTO users, got: %s", q)
	}
	// primary key should NOT appear in INSERT columns
	colsPart := q[strings.Index(q, "(")+1 : strings.Index(q, ")")]
	if strings.Contains(colsPart, "id") {
		t.Errorf("primary key 'id' should not appear in INSERT columns, got: %s", colsPart)
	}
}

func TestCreate_QueryShape_NoReturning_MySQL(t *testing.T) {
	ex := &mockExecutor{execResult: mockResult{lastID: 42}}
	c := newMySQLCRUD(ex)

	id, err := c.Create(&User{Name: "Ion", Email: "ion@test.com"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	q := ex.lastCall().query
	if strings.Contains(q, "RETURNING") {
		t.Errorf("MySQL query should NOT contain RETURNING, got: %s", q)
	}
	if id != "42" {
		t.Errorf("MySQL Create should return LastInsertId as string, got %q", id)
	}
}

func TestCreate_QueryShape_Returning_Postgres(t *testing.T) {
	// fakeDB returns ErrNoRows on Scan — not a panic, just an error.
	// We only care about the recorded query, not the scan result.
	ex := &mockExecutor{}
	c := newPostgresCRUD(ex)

	_, _ = c.Create(&User{Name: "Ion", Email: "ion@test.com"})

	if len(ex.calls) == 0 {
		t.Fatal("expected QueryRow to be called")
	}

	q := ex.calls[0].query
	if !strings.HasPrefix(q, "INSERT INTO users") {
		t.Errorf("query should start with INSERT INTO users, got: %s", q)
	}
	if !strings.Contains(q, "RETURNING id") {
		t.Errorf("Postgres query should contain RETURNING id, got: %s", q)
	}
}

func TestCreate_NoTableName_ReturnsError(t *testing.T) {
	ex := &mockExecutor{}
	c := newPostgresCRUD(ex)

	_, err := c.Create(&NoTableModel{Name: "test"})
	if err == nil {
		t.Fatal("expected error for model without TableName()")
	}
	if !errors.Is(err, ErrNoTableName) {
		t.Errorf("expected ErrNoTableName, got: %v", err)
	}
}

func TestCreate_SetsTimestamps(t *testing.T) {
	ex := &mockExecutor{execResult: mockResult{lastID: 1}}
	c := newMySQLCRUD(ex)

	before := time.Now()
	u := &User{Name: "Ion", Email: "ion@test.com"}
	_, _ = c.Create(u)
	after := time.Now()

	if u.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set after Create")
	}
	if u.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should be set after Create")
	}
	if u.CreatedAt.Before(before) || u.CreatedAt.After(after) {
		t.Error("CreatedAt should be within test time bounds")
	}
}

func TestCreate_ExecError_WrappedAsOpError(t *testing.T) {
	ex := &mockExecutor{execErr: errors.New("db connection lost")}
	c := newMySQLCRUD(ex)

	_, err := c.Create(&User{Name: "Ion", Email: "ion@test.com"})
	if err == nil {
		t.Fatal("expected error")
	}

	var opE *OpError
	if !errors.As(err, &opE) {
		t.Errorf("expected *OpError, got: %T — %v", err, err)
	}
	if opE.Op != "Create" {
		t.Errorf("OpError.Op = %q, want %q", opE.Op, "Create")
	}
	if opE.Table != "users" {
		t.Errorf("OpError.Table = %q, want %q", opE.Table, "users")
	}
}

// ── Update ────────────────────────────────────────────────────────────────────

func TestUpdate_QueryShape_Postgres(t *testing.T) {
	ex := &mockExecutor{}
	c := newPostgresCRUD(ex)

	_ = c.Update(&User{Name: "Ion Updated", Email: "ion@test.com"}, "id", "abc-123")

	q := ex.lastCall().query
	if !strings.HasPrefix(q, "UPDATE users SET") {
		t.Errorf("query should start with UPDATE users SET, got: %s", q)
	}
	if !strings.Contains(q, "WHERE id =") {
		t.Errorf("query should contain WHERE id =, got: %s", q)
	}
	setPart := q[strings.Index(q, "SET")+3 : strings.Index(q, "WHERE")]
	if strings.Contains(setPart, " id ") {
		t.Errorf("primary key should not appear in SET clause, got: %s", setPart)
	}
}

func TestUpdate_SetsUpdatedAt(t *testing.T) {
	ex := &mockExecutor{}
	c := newPostgresCRUD(ex)

	u := &User{Name: "Ion", Email: "ion@test.com"}
	before := time.Now()
	_ = c.Update(u, "id", "abc-123")
	after := time.Now()

	if u.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should be set after Update")
	}
	if u.UpdatedAt.Before(before) || u.UpdatedAt.After(after) {
		t.Error("UpdatedAt should be within test time bounds")
	}
}

func TestUpdate_ExecError_WrappedAsOpError(t *testing.T) {
	ex := &mockExecutor{execErr: errors.New("timeout")}
	c := newPostgresCRUD(ex)

	err := c.Update(&User{Name: "x", Email: "x@x.com"}, "id", "1")

	var opE *OpError
	if !errors.As(err, &opE) {
		t.Errorf("expected *OpError, got: %T", err)
	}
	if opE.Op != "Update" {
		t.Errorf("OpError.Op = %q, want Update", opE.Op)
	}
}

func TestUpdate_NoTableName_ReturnsError(t *testing.T) {
	ex := &mockExecutor{}
	c := newPostgresCRUD(ex)

	err := c.Update(&NoTableModel{Name: "x"}, "id", "1")
	if !errors.Is(err, ErrNoTableName) {
		t.Errorf("expected ErrNoTableName, got: %v", err)
	}
}

// ── Read ──────────────────────────────────────────────────────────────────────

func TestRead_QueryShape(t *testing.T) {
	ex := &mockExecutor{queryErr: errors.New("intentional — no rows")}
	c := newPostgresCRUD(ex)

	var dest []User
	_ = c.Read(&User{}, "email", "ion@test.com", &dest)

	q := ex.lastCall().query
	if !strings.HasPrefix(q, "SELECT * FROM users") {
		t.Errorf("query should start with SELECT * FROM users, got: %s", q)
	}
	if !strings.Contains(q, "WHERE email = $1") {
		t.Errorf("query should contain WHERE email = $1, got: %s", q)
	}
}

func TestRead_QueryError_WrappedAsOpError(t *testing.T) {
	ex := &mockExecutor{queryErr: errors.New("query failed")}
	c := newPostgresCRUD(ex)

	var dest []User
	err := c.Read(&User{}, "email", "x@x.com", &dest)

	var opE *OpError
	if !errors.As(err, &opE) {
		t.Errorf("expected *OpError, got: %T", err)
	}
	if opE.Op != "Read" {
		t.Errorf("OpError.Op = %q, want Read", opE.Op)
	}
}

func TestRead_NoTableName_ReturnsError(t *testing.T) {
	ex := &mockExecutor{}
	c := newPostgresCRUD(ex)

	var dest []NoTableModel
	err := c.Read(&NoTableModel{}, "name", "test", &dest)
	if !errors.Is(err, ErrNoTableName) {
		t.Errorf("expected ErrNoTableName, got: %v", err)
	}
}

// ── Get ───────────────────────────────────────────────────────────────────────

func TestGet_QueryShape_Postgres(t *testing.T) {
	ex := &mockExecutor{queryErr: errors.New("intentional")}
	c := newPostgresCRUD(ex)

	var dest []User
	_ = c.Get(&User{}, &dest, 0, 20)

	q := ex.lastCall().query
	if !strings.Contains(q, "SELECT * FROM users") {
		t.Errorf("query should contain SELECT * FROM users, got: %s", q)
	}
	if !strings.Contains(q, "LIMIT $1 OFFSET $2") {
		t.Errorf("query should contain LIMIT $1 OFFSET $2, got: %s", q)
	}
}

func TestGet_QueryShape_MySQL(t *testing.T) {
	ex := &mockExecutor{queryErr: errors.New("intentional")}
	c := newMySQLCRUD(ex)

	var dest []User
	_ = c.Get(&User{}, &dest, 0, 20)

	q := ex.lastCall().query
	if !strings.Contains(q, "LIMIT ? OFFSET ?") {
		t.Errorf("MySQL query should use ? placeholders, got: %s", q)
	}
}

func TestGet_PassesCorrectArgs(t *testing.T) {
	ex := &mockExecutor{queryErr: errors.New("intentional")}
	c := newPostgresCRUD(ex)

	var dest []User
	_ = c.Get(&User{}, &dest, 10, 25) // start=10, count=25

	args := ex.lastCall().args
	if len(args) != 2 {
		t.Fatalf("expected 2 args, got %d", len(args))
	}
	if args[0] != 25 {
		t.Errorf("args[0] (LIMIT/count) = %v, want 25", args[0])
	}
	if args[1] != 10 {
		t.Errorf("args[1] (OFFSET/start) = %v, want 10", args[1])
	}
}

// ── Delete ────────────────────────────────────────────────────────────────────

func TestDelete_HardDelete(t *testing.T) {
	ex := &mockExecutor{}
	c := newPostgresCRUD(ex)

	_ = c.Delete(&NoTagModel{}, "id", "abc")

	q := ex.lastCall().query
	if !strings.HasPrefix(q, "DELETE FROM no_tag_models") {
		t.Errorf("expected hard DELETE query, got: %s", q)
	}
}

func TestDelete_SoftDelete_UsesUpdate(t *testing.T) {
	ex := &mockExecutor{}
	c := newPostgresCRUD(ex)

	_ = c.Delete(&User{}, "id", "abc-123")

	q := ex.lastCall().query
	if strings.HasPrefix(q, "DELETE") {
		t.Errorf("User should use soft delete (UPDATE), got DELETE query: %s", q)
	}
	if !strings.HasPrefix(q, "UPDATE users SET deleted_at") {
		t.Errorf("soft delete query should UPDATE deleted_at, got: %s", q)
	}
	if !strings.Contains(q, "WHERE id =") {
		t.Errorf("soft delete query should filter by field, got: %s", q)
	}
}

func TestDelete_SoftDelete_ArgsCorrect(t *testing.T) {
	ex := &mockExecutor{}
	c := newPostgresCRUD(ex)

	_ = c.Delete(&User{}, "id", "abc-123")

	args := ex.lastCall().args
	if len(args) != 2 {
		t.Fatalf("soft delete should pass 2 args (timestamp, value), got %d", len(args))
	}
	if _, ok := args[0].(time.Time); !ok {
		t.Errorf("soft delete args[0] should be time.Time, got %T", args[0])
	}
	if args[1] != "abc-123" {
		t.Errorf("soft delete args[1] should be the where value, got %v", args[1])
	}
}

func TestDelete_ExecError_WrappedAsOpError(t *testing.T) {
	ex := &mockExecutor{execErr: errors.New("disk full")}
	c := newPostgresCRUD(ex)

	err := c.Delete(&NoTagModel{}, "id", "1")

	var opE *OpError
	if !errors.As(err, &opE) {
		t.Errorf("expected *OpError, got: %T", err)
	}
	if opE.Op != "Delete" {
		t.Errorf("OpError.Op = %q, want Delete", opE.Op)
	}
}

func TestDelete_NoTableName_ReturnsError(t *testing.T) {
	ex := &mockExecutor{}
	c := newPostgresCRUD(ex)

	err := c.Delete(&NoTableModel{}, "id", "1")
	if !errors.Is(err, ErrNoTableName) {
		t.Errorf("expected ErrNoTableName, got: %v", err)
	}
}
