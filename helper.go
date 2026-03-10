package crud

import (
	"database/sql"
	"fmt"
	"reflect"
	"strings"
	"unicode"
)

// parseColumn extracts the column name from a db tag.
// "email,notnull,unique" → "email"
// ""                     → ""
func parseColumn(tag string) string {
	if tag == "" {
		return ""
	}
	return strings.Split(tag, ",")[0]
}

// isPrimaryKey checks if the db tag contains "primary_key".
func isPrimaryKey(tag string) bool {
	parts := strings.Split(tag, ",")
	for _, p := range parts[1:] {
		if strings.TrimSpace(p) == "primary_key" {
			return true
		}
	}
	return false
}

// toSnakeCase converts a Go field name to snake_case.
// "UserID"    → "user_id"
// "CreatedAt" → "created_at"
// "Name"      → "name"
func toSnakeCase(s string) string {
	var b strings.Builder
	runes := []rune(s)

	for i, r := range runes {
		if unicode.IsUpper(r) {
			if i > 0 {
				prevLower := unicode.IsLower(runes[i-1])
				nextLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
				if prevLower || nextLower {
					b.WriteRune('_')
				}
			}
			b.WriteRune(unicode.ToLower(r))
		} else {
			b.WriteRune(r)
		}
	}

	return b.String()
}

// getTableName resolves the table name from a model.
// Returns ErrNoTableName if the model doesn't implement TableNamer.
func getTableName(model any) (string, error) {
	if t, ok := model.(TableNamer); ok {
		return t.TableName(), nil
	}

	val := reflect.Indirect(reflect.ValueOf(model))
	if val.CanInterface() {
		if t, ok := val.Interface().(TableNamer); ok {
			return t.TableName(), nil
		}
	}

	return "", fmt.Errorf("%w — model %T must implement: func (%T) TableName() string",
		ErrNoTableName, model, model,
	)
}

func scanRows(rows *sql.Rows, dest any) error {
	val := reflect.ValueOf(dest)
	if val.Kind() != reflect.Ptr || val.Elem().Kind() != reflect.Slice {
		return fmt.Errorf("dest must be pointer to slice")
	}

	slice := val.Elem()
	elemType := slice.Type().Elem()

	columns, err := rows.Columns()
	if err != nil {
		return err
	}

	// build column → field index map once per scan
	colIndex := make(map[string]int, elemType.NumField())
	for i := 0; i < elemType.NumField(); i++ {
		field := elemType.Field(i)
		rawTag := field.Tag.Get("db")
		col := parseColumn(rawTag)
		if col == "" {
			col = toSnakeCase(field.Name)
		}
		colIndex[col] = i
	}

	for rows.Next() {
		elem := reflect.New(elemType).Elem()

		values := make([]any, len(columns))
		ptrs := make([]any, len(columns))
		for i := range values {
			ptrs[i] = &values[i]
		}

		if err := rows.Scan(ptrs...); err != nil {
			return err
		}

		for j, col := range columns {
			fieldIdx, ok := colIndex[col]
			if !ok {
				continue
			}

			f := elem.Field(fieldIdx)
			if !f.CanSet() {
				continue
			}

			v := values[j]
			if b, ok := v.([]byte); ok {
				f.SetString(string(b))
			} else if v != nil {
				f.Set(reflect.ValueOf(v).Convert(f.Type()))
			}
		}

		slice.Set(reflect.Append(slice, elem))
	}

	return rows.Err()
}
