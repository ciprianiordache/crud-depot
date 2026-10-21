package crud

import (
	"context"
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

// WithTx returns a new CRUD instance that uses the given transaction.
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

// RunInTxContext is RunInTx with a context: the transaction is started
// with BeginTx(ctx, nil), so cancelling ctx rolls it back.
func (c *CRUD) RunInTxContext(ctx context.Context, db TxContextBeginner, fn func(tx *CRUD) error) (err error) {
	sqlTx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("crud-depot: begin transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = sqlTx.Rollback()
			panic(p)
		}
	}()

	if err = fn(c.WithTx(sqlTx)); err != nil {
		_ = sqlTx.Rollback()
		return fmt.Errorf("crud-depot: transaction rolled back: %w", err)
	}

	if err = sqlTx.Commit(); err != nil {
		return fmt.Errorf("crud-depot: commit transaction: %w", err)
	}

	return nil
}

func (c *CRUD) Create(model any) (string, error) {
	return c.CreateContext(context.Background(), model)
}

// CreateContext inserts model and returns its primary key.
func (c *CRUD) CreateContext(ctx context.Context, model any) (string, error) {
	table, err := getTableName(model)
	if err != nil {
		return "", err
	}

	db := c.ctxDB()
	meta := getMeta(model)
	val := reflect.ValueOf(model).Elem()
	now := time.Now()

	// apply all tag-driven values before building the query
	for _, f := range meta.fields {
		field := val.Field(f.index)

		switch {
		case f.autoGen:
			// SERIAL / AUTO_INCREMENT — DB generates, skip entirely
			continue

		case f.uuidGen:
			// uuid — crud-depot generates, set on the struct before INSERT
			if field.Kind() == reflect.String && field.IsZero() {
				id, err := generateUUID()
				if err != nil {
					return "", err
				}
				field.SetString(id)
			}

		case f.onCreate || f.onWrite:
			setTime(field, now)

		case f.defaultVal != "":
			applyDefault(field, f.defaultVal)
		}
	}

	var cols, placeholders []string
	var args []any

	for _, f := range meta.fields {
		// skip only SERIAL/AUTO_INCREMENT — uuid fields are now included with their generated value
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

	// for uuid PKs we don't need RETURNING — we already know the ID
	returning := ""
	if meta.primaryKey != nil && meta.primaryKey.autoGen {
		returning = c.dialect.ReturningClause(pkCol)
	}

	query := strings.TrimSpace(fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s) %s",
		table,
		strings.Join(cols, ", "),
		strings.Join(placeholders, ", "),
		returning,
	))

	// auto-increment path — DB generates the ID
	if meta.primaryKey != nil && meta.primaryKey.autoGen {
		if c.dialect.UsesLastInsertID() {
			result, err := db.ExecContext(ctx, query, args...)
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
		if err = db.QueryRowContext(ctx, query, args...).Scan(&id); err != nil {
			return "", opErr("Create", table, err)
		}
		return id, nil
	}

	// uuid path — crud-depot generated the ID, just execute and return it
	if _, err = db.ExecContext(ctx, query, args...); err != nil {
		return "", opErr("Create", table, err)
	}

	// return the UUID we set on the struct
	if meta.primaryKey != nil {
		return val.Field(meta.primaryKey.index).String(), nil
	}

	return "", nil
}

func (c *CRUD) Update(model any, whereField string, whereValue any) error {
	return c.UpdateContext(context.Background(), model, Eq(whereField, whereValue))
}

// UpdateContext writes every non-key field of model to the rows matching
// where. It returns ErrNotFound when no row matches.
//
//	err := depot.UpdateContext(ctx, &unit, crud.Eq("organization_id", orgID).And("id", unit.ID))
func (c *CRUD) UpdateContext(ctx context.Context, model any, where Where) error {
	table, err := getTableName(model)
	if err != nil {
		return err
	}

	meta := getMeta(model)
	val := reflect.ValueOf(model).Elem()

	now := time.Now()
	for _, f := range meta.fields {
		if f.onWrite {
			setTime(val.Field(f.index), now)
		}
	}

	var sets []string
	var args []any

	for _, f := range meta.fields {
		if f.primaryKey || f.autoGen || f.uuidGen {
			continue
		}
		args = append(args, val.Field(f.index).Interface())
		sets = append(sets, fmt.Sprintf("%s = %s", f.column, c.dialect.Placeholder(len(args))))
	}

	cond, condArgs, err := where.clause(c.dialect, len(args))
	if err != nil {
		return opErr("Update", table, err)
	}

	query := fmt.Sprintf(
		"UPDATE %s SET %s WHERE %s",
		table,
		strings.Join(sets, ", "),
		cond,
	)

	result, err := c.ctxDB().ExecContext(ctx, query, append(args, condArgs...)...)
	if err != nil {
		return opErr("Update", table, err)
	}

	n, err := result.RowsAffected()
	if err == nil && n == 0 {
		return ErrNotFound
	}

	return nil
}

func (c *CRUD) Read(model any, field string, value any, dest any) error {
	return c.ReadContext(context.Background(), model, Eq(field, value), dest)
}

// ReadContext scans every row matching where into dest (a pointer to a
// slice). It returns ErrNotFound when no row matches.
func (c *CRUD) ReadContext(ctx context.Context, model any, where Where, dest any) error {
	table, err := getTableName(model)
	if err != nil {
		return err
	}

	cond, args, err := where.clause(c.dialect, 0)
	if err != nil {
		return opErr("Read", table, err)
	}

	query := fmt.Sprintf("SELECT * FROM %s WHERE %s", table, cond)

	rows, err := c.ctxDB().QueryContext(ctx, query, args...)
	if err != nil {
		return opErr("Read", table, err)
	}
	defer rows.Close()

	if err = scanRows(rows, dest); err != nil {
		return opErr("Read", table, err)
	}

	// check if any rows were returned
	destVal := reflect.ValueOf(dest).Elem()
	if destVal.Kind() == reflect.Slice && destVal.Len() == 0 {
		return ErrNotFound
	}

	return nil
}

// ReadOne returns a single record matching field = value.
// Returns ErrNotFound if no row matches.
//
// Example:
//
//	var user model.User
//	err := depot.ReadOne(model.User{}, "email", "ion@example.com", &user)
//	if errors.Is(err, crud.ErrNotFound) {
//	    // user does not exist
//	}
func (c *CRUD) ReadOne(model any, field string, value any, dest any) error {
	return c.ReadOneContext(context.Background(), model, Eq(field, value), dest)
}

// ReadOneContext returns the first record matching where, or ErrNotFound.
//
//	var u unit.Unit
//	err := depot.ReadOneContext(ctx, unit.Unit{}, crud.Eq("organization_id", orgID).And("id", id), &u)
func (c *CRUD) ReadOneContext(ctx context.Context, model any, where Where, dest any) error {
	table, err := getTableName(model)
	if err != nil {
		return err
	}

	// scan into a temporary slice then extract the first element
	val := reflect.ValueOf(dest)
	if val.Kind() != reflect.Ptr {
		return opErr("ReadOne", table, fmt.Errorf("dest must be a pointer to a struct"))
	}

	cond, args, err := where.clause(c.dialect, 0)
	if err != nil {
		return opErr("ReadOne", table, err)
	}

	query := fmt.Sprintf("SELECT * FROM %s WHERE %s LIMIT 1", table, cond)

	rows, err := c.ctxDB().QueryContext(ctx, query, args...)
	if err != nil {
		return opErr("ReadOne", table, err)
	}
	defer rows.Close()

	elemType := val.Elem().Type()
	slicePtr := reflect.New(reflect.SliceOf(elemType))

	if err = scanRows(rows, slicePtr.Interface()); err != nil {
		return opErr("ReadOne", table, err)
	}

	slice := slicePtr.Elem()
	if slice.Len() == 0 {
		return ErrNotFound
	}

	val.Elem().Set(slice.Index(0))
	return nil
}

func (c *CRUD) Get(model any, dest any, start, count int) error {
	return c.GetContext(context.Background(), model, dest, start, count)
}

// GetContext reads count rows starting at start, in no particular order.
func (c *CRUD) GetContext(ctx context.Context, model any, dest any, start, count int) error {
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

	rows, err := c.ctxDB().QueryContext(ctx, query, count, start)
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
	return c.DeleteContext(context.Background(), model, Eq(field, value))
}

// DeleteContext deletes the rows matching where - or, for a SoftDeleter,
// sets their soft-delete column to now. It returns ErrNotFound when no row
// matches.
func (c *CRUD) DeleteContext(ctx context.Context, model any, where Where) error {
	table, err := getTableName(model)
	if err != nil {
		return err
	}
	db := c.ctxDB()

	if sd, ok := model.(SoftDeleter); ok {
		col := sd.SoftDeleteField()
		cond, args, err := where.clause(c.dialect, 1)
		if err != nil {
			return opErr("Delete(soft)", table, err)
		}
		query := fmt.Sprintf(
			"UPDATE %s SET %s = %s WHERE %s",
			table, col,
			c.dialect.Placeholder(1),
			cond,
		)
		result, err := db.ExecContext(ctx, query, append([]any{time.Now()}, args...)...)
		if err != nil {
			return opErr("Delete(soft)", table, err)
		}
		n, err := result.RowsAffected()
		if err == nil && n == 0 {
			return ErrNotFound
		}
		return nil
	}

	cond, args, err := where.clause(c.dialect, 0)
	if err != nil {
		return opErr("Delete", table, err)
	}
	query := fmt.Sprintf("DELETE FROM %s WHERE %s", table, cond)

	result, err := db.ExecContext(ctx, query, args...)
	if err != nil {
		return opErr("Delete", table, err)
	}
	n, err := result.RowsAffected()
	if err == nil && n == 0 {
		return ErrNotFound
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
			autoGen:    hasOption(rawTag, "auto"), // SERIAL/AUTO_INCREMENT — DB generates
			uuidGen:    hasOption(rawTag, "uuid"), // UUID — crud-depot generates
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
