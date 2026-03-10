package crud

import (
	"errors"
	"fmt"
)

// ErrNoTableName is returned when a model does not implement TableNamer.
var ErrNoTableName = errors.New("crud-depot: model does not implement TableNamer")

// ErrInvalidDest is returned when the dest argument is not a pointer to a slice.
var ErrInvalidDest = errors.New("crud-depot: dest must be a pointer to a slice")

// OpError wraps an underlying SQL error with operation context.
type OpError struct {
	Op    string // "Create", "Read", "Update", "Delete", "Get"
	Table string
	Err   error
}

func (e *OpError) Error() string {
	return fmt.Sprintf("crud-depot: %s on %q failed: %v", e.Op, e.Table, e.Err)
}

func (e *OpError) Unwrap() error {
	return e.Err
}

func opErr(op, table string, err error) error {
	return &OpError{Op: op, Table: table, Err: err}
}
