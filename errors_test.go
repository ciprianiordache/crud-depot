package crud

import (
	"errors"
	"strings"
	"testing"
)

func TestOpError_ErrorMessage(t *testing.T) {
	inner := errors.New("connection refused")
	err := opErr("Create", "users", inner)

	msg := err.Error()
	if !strings.Contains(msg, "Create") {
		t.Errorf("error message should contain op name, got: %s", msg)
	}
	if !strings.Contains(msg, "users") {
		t.Errorf("error message should contain table name, got: %s", msg)
	}
	if !strings.Contains(msg, "connection refused") {
		t.Errorf("error message should contain original error, got: %s", msg)
	}
}

func TestOpError_Unwrap(t *testing.T) {
	inner := errors.New("original error")
	err := opErr("Update", "orders", inner)

	if !errors.Is(err, inner) {
		t.Error("errors.Is should find the wrapped error via Unwrap")
	}
}

func TestOpError_As(t *testing.T) {
	err := opErr("Delete", "sessions", errors.New("timeout"))

	var opE *OpError
	if !errors.As(err, &opE) {
		t.Fatal("errors.As should match *OpError")
	}
	if opE.Op != "Delete" {
		t.Errorf("Op = %q, want Delete", opE.Op)
	}
	if opE.Table != "sessions" {
		t.Errorf("Table = %q, want sessions", opE.Table)
	}
}

func TestErrNoTableName_IsSentinel(t *testing.T) {
	err := errors.New("some other error")
	if errors.Is(err, ErrNoTableName) {
		t.Error("unrelated error should not match ErrNoTableName")
	}

	wrapped := errors.Join(ErrNoTableName, errors.New("extra context"))
	if !errors.Is(wrapped, ErrNoTableName) {
		t.Error("wrapped ErrNoTableName should be detectable with errors.Is")
	}
}
