package crud

import (
	"errors"
	"testing"
)

func TestWithTx_UsesDifferentExecutor(t *testing.T) {
	originalEx := &mockExecutor{}
	txEx := &mockExecutor{}

	c := newPostgresCRUD(originalEx)
	cTx := c.WithTx(txEx)

	// WithTx should return a NEW CRUD pointing to txEx
	if cTx.db != txEx {
		t.Error("WithTx should use the provided executor")
	}
	if cTx.db == originalEx {
		t.Error("WithTx should not use the original executor")
	}
}

func TestWithTx_PreservesDialect(t *testing.T) {
	ex := &mockExecutor{}
	txEx := &mockExecutor{}

	c := &CRUD{db: ex, dialect: Postgres{}}
	cTx := c.WithTx(txEx)

	if _, ok := cTx.dialect.(Postgres); !ok {
		t.Error("WithTx should preserve the original dialect")
	}
}

func TestWithTx_OriginalUnchanged(t *testing.T) {
	ex := &mockExecutor{}
	txEx := &mockExecutor{}

	c := newPostgresCRUD(ex)
	_ = c.WithTx(txEx)

	// original should still point to ex
	if c.db != ex {
		t.Error("WithTx should not mutate the original CRUD instance")
	}
}

func TestWithTx_OperationsUseTransactionExecutor(t *testing.T) {
	originalEx := &mockExecutor{}
	txEx := &mockExecutor{execErr: errors.New("tx error")}

	c := newMySQLCRUD(originalEx)
	cTx := c.WithTx(txEx)

	err := cTx.Update(&User{Name: "x", Email: "x@x.com"}, "id", "1")

	// error should come from txEx, not originalEx
	if err == nil {
		t.Fatal("expected error from txEx executor")
	}
	if len(originalEx.calls) > 0 {
		t.Error("original executor should not have been called")
	}
	if len(txEx.calls) == 0 {
		t.Error("tx executor should have been called")
	}
}
