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
	autoGen    bool // DB generates the value (serial, uuid, etc.) — skip on INSERT
}

type modelMeta struct {
	table      string
	fields     []fieldMeta
	primaryKey *fieldMeta
}

var metaCache sync.Map
