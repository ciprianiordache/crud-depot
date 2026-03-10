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
}

type modelMeta struct {
	table      string
	fields     []fieldMeta
	primaryKey *fieldMeta
}

var metaCache sync.Map
