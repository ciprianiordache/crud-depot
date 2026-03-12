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
	autoGen    bool   // auto (SERIAL/AUTO_INCREMENT) — skip on INSERT, DB generates
	uuidGen    bool   // uuid — crud-depot generates UUID before INSERT
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
