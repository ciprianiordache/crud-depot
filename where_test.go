package crud

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
)

func TestWhere_ClausePlaceholdersContinue(t *testing.T) {
	w := Eq("organization_id", "org").And("id", "u1").And("deleted_at", nil)
	cond, args, err := w.clause(Postgres{}, 3)
	if err != nil {
		t.Fatal(err)
	}
	if cond != "organization_id = $4 AND id = $5 AND deleted_at IS NULL" {
		t.Fatalf("cond = %q", cond)
	}
	if len(args) != 2 || args[0] != "org" || args[1] != "u1" {
		t.Fatalf("args = %v", args)
	}

	cond, _, _ = w.clause(MySQL{}, 3)
	if cond != "organization_id = ? AND id = ? AND deleted_at IS NULL" {
		t.Fatalf("mysql cond = %q", cond)
	}
}

func TestWhere_AndDoesNotAlias(t *testing.T) {
	base := Eq("organization_id", "org")
	a := base.And("id", "a")
	b := base.And("id", "b")
	if a[1].Value != "a" || b[1].Value != "b" || len(base) != 1 {
		t.Fatalf("And shares storage: a=%v b=%v base=%v", a, b, base)
	}
}

func TestUpdateContext_TenantScopedQuery(t *testing.T) {
	ex := &mockExecutor{}
	c := newPostgresCRUD(ex)

	u := &User{ID: "7", Name: "Ion", Email: "ion@test.com"}
	if err := c.UpdateContext(context.Background(), u, Eq("organization_id", "org").And("id", u.ID)); err != nil {
		t.Fatal(err)
	}
	q := ex.lastCall()
	if !strings.HasSuffix(q.query, "WHERE organization_id = $7 AND id = $8") {
		t.Fatalf("query = %s", q.query)
	}
	if n := len(q.args); n != 8 || q.args[6] != "org" || q.args[7] != "7" {
		t.Fatalf("args = %v", q.args)
	}
	if u.UpdatedAt.IsZero() {
		t.Fatal("UpdatedAt should be set")
	}
}

func TestWhere_EmptyOrInvalidRunsNoQuery(t *testing.T) {
	ctx := context.Background()
	cases := map[string]Where{
		"empty":     nil,
		"injection": Eq("id = id OR 1", 1),
		"blank":     Eq("", 1),
	}
	for name, w := range cases {
		ex := &mockExecutor{}
		c := newPostgresCRUD(ex)
		var dest []User
		errs := []error{
			c.UpdateContext(ctx, &User{}, w),
			c.ReadContext(ctx, User{}, w, &dest),
			c.ReadOneContext(ctx, User{}, w, &User{}),
			c.DeleteContext(ctx, User{}, w),
		}
		for i, err := range errs {
			if err == nil {
				t.Errorf("%s: call %d: want an error", name, i)
			}
		}
		if name == "empty" && !errors.Is(errs[0], ErrEmptyWhere) {
			t.Errorf("empty: err = %v, want ErrEmptyWhere", errs[0])
		}
		if len(ex.calls) != 0 {
			t.Errorf("%s: %d queries ran: %v", name, len(ex.calls), ex.calls)
		}
	}
}

func TestDeleteContext_SoftDeletePlaceholders(t *testing.T) {
	ex := &mockExecutor{}
	c := newPostgresCRUD(ex)

	if err := c.DeleteContext(context.Background(), User{}, Eq("organization_id", "org").And("id", "7")); err != nil {
		t.Fatal(err)
	}
	q := ex.lastCall()
	if q.query != "UPDATE users SET deleted_at = $1 WHERE organization_id = $2 AND id = $3" {
		t.Fatalf("query = %s", q.query)
	}
	if len(q.args) != 3 || q.args[1] != "org" || q.args[2] != "7" {
		t.Fatalf("args = %v", q.args)
	}
}

func TestContext_ReachesTheDriver(t *testing.T) {
	db, err := sql.Open("crud_nullfake", "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	c := New(db, Postgres{})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var got []person
	if err := c.ReadContext(ctx, person{}, Eq("name", "Ana"), &got); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled context: err = %v, want context.Canceled", err)
	}

	// The old API keeps working on the same executor.
	if err := c.Read(person{}, "name", "Ana", &got); err != nil || len(got) != 2 {
		t.Fatalf("Read: %d rows, err %v", len(got), err)
	}
}
