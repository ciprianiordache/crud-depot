package crud

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unicode"
)

func parseColumn(tag string) string {
	if tag == "" {
		return ""
	}
	return strings.Split(tag, ",")[0]
}

func isPrimaryKey(tag string) bool { return hasOption(tag, "primary_key") }
func isAutoGen(tag string) bool    { return hasOption(tag, "auto") || hasOption(tag, "uuid") }

func getDefault(tag string) string {
	parts := strings.Split(tag, ",")
	for _, p := range parts[1:] {
		if strings.HasPrefix(p, "default:") {
			return strings.TrimPrefix(p, "default:")
		}
	}
	return ""
}

func hasOption(tag, option string) bool {
	parts := strings.Split(tag, ",")
	for _, p := range parts[1:] {
		if strings.TrimSpace(p) == option {
			return true
		}
	}
	return false
}

// applyDefault sets the field to the parsed default value
// only if the field is currently a zero value.
func applyDefault(f reflect.Value, raw string) {
	if !f.CanSet() || !f.IsZero() {
		return
	}

	switch f.Kind() {
	case reflect.String:
		f.SetString(raw)
	case reflect.Bool:
		if v, err := strconv.ParseBool(raw); err == nil {
			f.SetBool(v)
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if v, err := strconv.ParseInt(raw, 10, 64); err == nil {
			f.SetInt(v)
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if v, err := strconv.ParseUint(raw, 10, 64); err == nil {
			f.SetUint(v)
		}
	case reflect.Float32, reflect.Float64:
		if v, err := strconv.ParseFloat(raw, 64); err == nil {
			f.SetFloat(v)
		}
	}
}

// setTime sets a time.Time field to now, only if the field is settable.
func setTime(f reflect.Value, now time.Time) {
	if f.CanSet() && f.Type() == reflect.TypeOf(time.Time{}) {
		f.Set(reflect.ValueOf(now))
	}
}

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

// generateUUID generates a random UUID v4 using crypto/rand.
// Format: xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx
func generateUUID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("crud-depot: generate uuid: %w", err)
	}

	// set version 4
	b[6] = (b[6] & 0x0f) | 0x40
	// set variant bits (10xx)
	b[8] = (b[8] & 0x3f) | 0x80

	return fmt.Sprintf(
		"%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:],
	), nil
}
