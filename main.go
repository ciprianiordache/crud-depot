package crud

import (
	"fmt"
	"reflect"
	"strings"
	"time"
)

func New(db Executor, dialect Dialect) *CRUD {
	return &CRUD{
		db:      db,
		dialect: dialect,
	}
}

// WithTx returns a new CRUD instance that uses the gicen transaction.
// The caller is responsible for Commit and Rollback.
//
// Example:
//
// tx, err := db.Begin()
// if err != nil {...}
//
// crudTx := crud.WithTx(tx)
//
//	if _, err := crudTx.Create(&User{...}); err != nil {
//			tx.Rollback()
//			return err
//	}
//
// tx.Commit()
func (c *CRUD) WithTx(tx Executor) *CRUD {
	return &CRUD{
		db:      tx,
		dialect: c.dialect,
	}
}

// RunInTx runs fn inside a transaction that is automatically
// committed on success or rolled back on error or panic.
// db must implement TxBeginner (i.e. *sql.DB).
//
// Example:
//
//	err := depot.RunInTx(db, func(tx *CRUD) error {
//	    if _, err := tx.Create(&User{...}); err != nil {
//	        return err // triggers rollback
//	    }
//	    if _, err := tx.Create(&Profile{...}); err != nil {
//	        return err // triggers rollback
//	    }
//	    return nil // triggers commit
//	})
func (c *CRUD) RunInTx(db TxBeginner, fn func(tx *CRUD) error) (err error) {
	sqlTx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("crud-depot: begin transaction: %w", err)
	}

	// ensure rollback on panic
	defer func() {
		if p := recover(); p != nil {
			_ = sqlTx.Rollback()
			panic(p)
		}
	}()

	crudTx := c.WithTx(sqlTx)

	if err = fn(crudTx); err != nil {
		_ = sqlTx.Rollback()
		return fmt.Errorf("crud-depot: transaction rolled back: %w", err)
	}

	if err = sqlTx.Commit(); err != nil {
		return fmt.Errorf("crud-depot: commit transaction: %w", err)
	}

	return nil
}

func (c *CRUD) Create(model any) (string, error) {
	table, err := getTableName(model)
	if err != nil {
		return "", err
	}

	meta := getMeta(model)
	val := reflect.ValueOf(model).Elem()
	now := time.Now()

	for _, f := range meta.fields {
		if f.autoGen {
			continue
		}
		field := val.Field(f.index)

		// oncreate and onwrite — set time.Now()
		if f.onCreate || f.onWrite {
			setTime(field, now)
			continue
		}

		// default:X — set if field is zero value
		if f.defaultVal != "" {
			applyDefault(field, f.defaultVal)
		}
	}

	var cols, placeholders []string
	var args []any

	for _, f := range meta.fields {
		if f.autoGen {
			continue
		}
		cols = append(cols, f.column)
		args = append(args, val.Field(f.index).Interface())
		placeholders = append(placeholders, c.dialect.Placeholder(len(args)))
	}

	pkCol := "id"
	if meta.primaryKey != nil {
		pkCol = meta.primaryKey.column
	}

	query := strings.TrimSpace(fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s) %s",
		table,
		strings.Join(cols, ", "),
		strings.Join(placeholders, ", "),
		c.dialect.ReturningClause(pkCol),
	))

	if c.dialect.UsesLastInsertID() {
		result, err := c.db.Exec(query, args...)
		if err != nil {
			return "", opErr("Create", table, err)
		}
		id, err := result.LastInsertId()
		if err != nil {
			return "", opErr("Create", table, err)
		}
		return fmt.Sprintf("%d", id), nil
	}

	var id string
	if err = c.db.QueryRow(query, args...).Scan(&id); err != nil {
		return "", opErr("Create", table, err)
	}
	return id, nil
}

func (c *CRUD) Update(model any, whereField string, whereValue any) error {
	table, err := getTableName(model)
	if err != nil {
		return err
	}

	meta := getMeta(model)
	val := reflect.ValueOf(model).Elem()

	// update onwrite fields
	now := time.Now()
	for _, f := range meta.fields {
		if f.onWrite {
			setTime(val.Field(f.index), now)
		}
	}

	var sets []string
	var args []any

	for _, f := range meta.fields {
		if f.primaryKey || f.autoGen {
			continue
		}
		args = append(args, val.Field(f.index).Interface())
		sets = append(sets, fmt.Sprintf("%s = %s", f.column, c.dialect.Placeholder(len(args))))
	}

	args = append(args, whereValue)

	query := fmt.Sprintf(
		"UPDATE %s SET %s WHERE %s = %s",
		table,
		strings.Join(sets, ", "),
		whereField,
		c.dialect.Placeholder(len(args)),
	)

	if _, err = c.db.Exec(query, args...); err != nil {
		return opErr("Update", table, err)
	}
	return nil
}

func (c *CRUD) Read(model any, field string, value any, dest any) error {
	table, err := getTableName(model)
	if err != nil {
		return err
	}

	query := fmt.Sprintf(
		"SELECT * FROM %s WHERE %s = %s",
		table, field, c.dialect.Placeholder(1),
	)

	rows, err := c.db.Query(query, value)
	if err != nil {
		return opErr("Read", table, err)
	}
	defer rows.Close()

	if err = scanRows(rows, dest); err != nil {
		return opErr("Read", table, err)
	}
	return nil
}

func (c *CRUD) Get(model any, dest any, start, count int) error {
	table, err := getTableName(model)
	if err != nil {
		return err
	}

	query := fmt.Sprintf(
		"SELECT * FROM %s LIMIT %s OFFSET %s",
		table,
		c.dialect.Placeholder(1),
		c.dialect.Placeholder(2),
	)

	rows, err := c.db.Query(query, count, start)
	if err != nil {
		return opErr("Get", table, err)
	}
	defer rows.Close()

	if err = scanRows(rows, dest); err != nil {
		return opErr("Get", table, err)
	}
	return nil
}

func (c *CRUD) Delete(model any, field string, value any) error {
	table, err := getTableName(model)
	if err != nil {
		return err
	}

	if sd, ok := model.(SoftDeleter); ok {
		col := sd.SoftDeleteField()
		query := fmt.Sprintf(
			"UPDATE %s SET %s = %s WHERE %s = %s",
			table, col,
			c.dialect.Placeholder(1),
			field,
			c.dialect.Placeholder(2),
		)
		if _, err = c.db.Exec(query, time.Now(), value); err != nil {
			return opErr("Delete(soft)", table, err)
		}
		return nil
	}

	query := fmt.Sprintf(
		"DELETE FROM %s WHERE %s = %s",
		table, field, c.dialect.Placeholder(1),
	)

	if _, err = c.db.Exec(query, value); err != nil {
		return opErr("Delete", table, err)
	}
	return nil
}

func getMeta(model any) *modelMeta {
	t := reflect.TypeOf(model)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	if cached, ok := metaCache.Load(t); ok {
		return cached.(*modelMeta)
	}

	meta := buildMeta(t)
	metaCache.Store(t, meta)
	return meta
}

func buildMeta(t reflect.Type) *modelMeta {
	meta := &modelMeta{}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		rawTag := field.Tag.Get("db")

		col := parseColumn(rawTag)
		if col == "" {
			col = toSnakeCase(field.Name)
		}

		fm := fieldMeta{
			column:     col,
			index:      i,
			primaryKey: isPrimaryKey(rawTag),
			autoGen:    isAutoGen(rawTag) || isPrimaryKey(rawTag),
			defaultVal: getDefault(rawTag),
			onCreate:   hasOption(rawTag, "oncreate"),
			onWrite:    hasOption(rawTag, "onwrite"),
		}

		meta.fields = append(meta.fields, fm)

		if fm.primaryKey && meta.primaryKey == nil {
			meta.primaryKey = &meta.fields[len(meta.fields)-1]
		}
	}

	// fallback primary key: first field named "id"
	if meta.primaryKey == nil {
		for i := range meta.fields {
			if meta.fields[i].column == "id" {
				meta.fields[i].autoGen = true
				meta.primaryKey = &meta.fields[i]
				break
			}
		}
	}

	return meta
}
