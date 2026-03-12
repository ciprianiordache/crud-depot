package crud

import "sync"

type CRUD struct {
	db      Executor
	dialect Dialect
}

type Postgres struct{}
type MySQL struct{}
type SQLite struct{}

type fieldMeta struct {
	column     string
	index      int
	primaryKey bool
	autoGen    bool   // uuid/auto — skip on INSERT, DB generates
	defaultVal string // default:X — applied at Create if field is zero
	onCreate   bool   // oncreate — set time.Now() at Create
	onWrite    bool   // onwrite  — set time.Now() at Create and Update
}

type modelMeta struct {
	table      string
	fields     []fieldMeta
	primaryKey *fieldMeta
}

var metaCache sync.Map
