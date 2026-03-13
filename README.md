[![Go Reference](https://pkg.go.dev/badge/github.com/ciprianiordache/crud-depot.svg)](https://pkg.go.dev/github.com/ciprianiordache/crud-depot)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

# CRUD Depot

A lightweight, reusable CRUD helper for Go applications.

**CRUD Depot** is a small utility designed to simplify common database operations while staying minimal and predictable.

It is **not an ORM**.
It provides a thin layer on top of `database/sql` to perform CRUD operations using Go structs and struct tags.

The goal is to make database access:

- simple
- reusable
- predictable
- portable across multiple projects

---

## Install

```bash
go get -u github.com/ciprianiordache/crud-depot
```

---

## Why This Library Exists

Most Go projects eventually repeat the same logic:

- mapping struct fields to SQL columns
- building `INSERT` and `UPDATE` queries
- scanning rows into structs
- handling transactions
- managing timestamps
- implementing soft deletes

This library removes that boilerplate while **still letting you control your SQL architecture**.

---

## Features

- Simple CRUD operations
- Supports `*sql.DB` and `*sql.Tx`
- Multiple SQL dialects (Postgres, MySQL, SQLite)
- Struct → column mapping via `db` tags
- Metadata caching for performance (reflection runs once per type)
- Automatic default values via struct tags
- Automatic timestamps via `oncreate` / `onwrite` tags
- Soft delete support via `SoftDeleter` interface
- Transaction helpers (`WithTx`, `RunInTx`)
- Descriptive, wrappable errors (`OpError`)
- Small and predictable API

---

## Supported Dialects

| Dialect  | Placeholder | Returning ID         |
|----------|-------------|----------------------|
| Postgres | `$1, $2`    | `RETURNING id`       |
| MySQL    | `?`         | `LastInsertId()`     |
| SQLite   | `?`         | `LastInsertId()`     |

---

# Usage

## Define a Model

```go
package model

import "time"

type User struct {
    ID        string    `db:"id,primary_key,uuid"`
    Name      string    `db:"name,notnull"`
    Email     string    `db:"email,notnull,unique,index"`
    Role      string    `db:"role,notnull,default:operator"`
    Active    bool      `db:"active,default:false"`
    CreatedAt time.Time `db:"created_at,oncreate"`
    UpdatedAt time.Time `db:"updated_at,onwrite"`
}

func (User) TableName() string {
    return "users"
}
```

---

## Create a CRUD Instance

```go
// Postgres
db, _ := sql.Open("postgres", dsn)
depot := crud.New(db, crud.Postgres{})

// MySQL
db, _ := sql.Open("mysql", dsn)
depot := crud.New(db, crud.MySQL{})

// SQLite
db, _ := sql.Open("sqlite3", "./app.db")
depot := crud.New(db, crud.SQLite{})
```

---

## Operations

### Create

```go
user := &model.User{
    Name:  "Ion",
    Email: "ion@example.com",
}

id, err := depot.Create(user)
```

Before INSERT, crud-depot applies all tag-driven defaults:

- `uuid` → ID is skipped, DB or application generates it
- `default:operator` → Role is set to `"operator"` if zero
- `default:false` → Active is set to `false` if zero  
- `oncreate` → CreatedAt is set to `time.Now()`
- `onwrite` → UpdatedAt is set to `time.Now()`

---

### Read

```go
var users []model.User

err := depot.Read(
    &model.User{},
    "email",
    "ion@example.com",
    &users,
)
```

---

### ReadOne

Returns a single record. Returns `ErrNotFound` if no row matches — no need to check for empty slices.

```go
var user model.User

err := depot.ReadOne(
    model.User{},
    "email",
    "ion@example.com",
    &user,
)
if errors.Is(err, crud.ErrNotFound) {
    // user does not exist
}
```

---

### Get (pagination)

```go
var users []model.User

err := depot.Get(
    &model.User{},
    &users,
    0,  // offset
    20, // limit
)
```

---

### Update

```go
user.Name = "Ion Updated"

err := depot.Update(
    user,
    "id",
    user.ID,
)
```

At `Update`, crud-depot automatically updates all fields tagged with `onwrite`.

---

### Delete

```go
err := depot.Delete(
    &model.User{},
    "id",
    userID,
)
```

If the model implements `SoftDeleter`, `Delete` issues an `UPDATE` instead of `DELETE`.

---

## Struct Tag Reference

| Tag                    | Behavior                                                                 |
|------------------------|--------------------------------------------------------------------------|
| `db:"column_name"`     | Maps field to SQL column                                                 |
| `primary_key`          | Marks field as primary key — skipped on INSERT and UPDATE                |
| `uuid`                 | Primary key is a UUID string — skipped on INSERT, returned via RETURNING |
| `auto`                 | Primary key is auto-increment (SERIAL / AUTO_INCREMENT)                  |
| `notnull`              | Informational — used by schema-builder for NOT NULL constraint           |
| `unique`               | Informational — used by schema-builder for UNIQUE constraint             |
| `index`                | Informational — used by schema-builder to create an index                |
| `default:value`        | Sets field to `value` at Create if the field is a zero value             |
| `oncreate`             | Sets `time.Time` field to `time.Now()` at Create                         |
| `onwrite`              | Sets `time.Time` field to `time.Now()` at Create and Update              |

If no `db` tag is present, the field name is converted to **snake_case** automatically:

```
CreatedAt → created_at
UserID    → user_id
FirstName → first_name
```

---

## Default Values

Defaults are applied by crud-depot at runtime — not by the database.

```go
type User struct {
    Role   string    `db:"role,default:operator"`       // "operator" if zero
    Active bool      `db:"active,default:false"`        // false if zero
    Score  int       `db:"score,default:100"`           // 100 if zero
}
```

Supported types: `string`, `bool`, `int`, `int64`, `float64` and their variants.

For timestamps, use `oncreate` / `onwrite` instead of `default:`.

---

## Timestamps

```go
type User struct {
    CreatedAt time.Time `db:"created_at,oncreate"` // set once at Create
    UpdatedAt time.Time `db:"updated_at,onwrite"`  // set at Create and Update
}
```

No interfaces to implement. Everything is driven by the tag.

---

## Soft Delete

Implement the `SoftDeleter` interface on your model:

```go
func (User) SoftDeleteField() string {
    return "deleted_at"
}
```

`Delete()` then issues:

```sql
UPDATE users SET deleted_at = $1 WHERE id = $2
```

instead of a physical `DELETE`.

---

## Transactions

### RunInTx — automatic commit / rollback

```go
err := depot.RunInTx(db, func(tx *crud.CRUD) error {
    userID, err := tx.Create(&model.User{Name: "Ion"})
    if err != nil {
        return err // triggers rollback
    }

    _, err = tx.Create(&model.Profile{UserID: userID})
    if err != nil {
        return err // triggers rollback
    }

    return nil // triggers commit
})
```

### WithTx — manual control

```go
tx, err := db.Begin()
if err != nil { ... }

crudTx := depot.WithTx(tx)

if _, err := crudTx.Create(&model.User{Name: "Ion"}); err != nil {
    tx.Rollback()
    return err
}

tx.Commit()
```

---

## Error Handling

All operations return descriptive, wrappable errors.

```go
id, err := depot.Create(&model.User{})
if err != nil {
    // check operation and table
    var opErr *crud.OpError
    if errors.As(err, &opErr) {
        log.Printf("operation %s on table %s failed", opErr.Op, opErr.Table)
    }

    // check sentinel errors
    if errors.Is(err, crud.ErrNoTableName) {
        log.Println("model does not implement TableName()")
    }
    if errors.Is(err, crud.ErrNotFound) {
        log.Println("record not found")
    }
}
```

`OpError` structure:

```go
type OpError struct {
    Op    string // "Create", "Read", "Update", "Delete", "Get"
    Table string
    Err   error
}
```

---

## Struct Mapping

Column names are resolved in this order:

1. `db` tag column name — `db:"email"` → `email`
2. snake_case of field name — `CreatedAt` → `created_at`

```go
type User struct {
    Email     string    `db:"email"`       // → "email"
    CreatedAt time.Time                    // → "created_at" (fallback)
    UserID    string                       // → "user_id" (fallback)
}
```

---

## Performance

To minimize reflection overhead:

- model metadata is built once and cached in a `sync.Map`
- subsequent operations reuse the cached metadata
- reflection runs **once per model type**, not per request

---

## Recommended Usage

- Keep complex queries inside **repository structs**
- Use this tool only for **simple CRUD operations**
- Write raw SQL for joins, aggregations, and complex filters
- Use `RunInTx` when performing multiple related writes

Example repository:

```go
type UserRepository struct {
    depot *crud.CRUD
}

func NewUserRepo(db crud.Executor) *UserRepository {
    return &UserRepository{
        depot: crud.New(db, crud.Postgres{}),
    }
}

func (r *UserRepository) Create(user *model.User) (string, error) {
    return r.depot.Create(user)
}

func (r *UserRepository) FindByEmail(email string) (*model.User, error) {
    var users []model.User
    if err := r.depot.Read(&model.User{}, "email", email, &users); err != nil {
        return nil, err
    }
    if len(users) == 0 {
        return nil, fmt.Errorf("user not found")
    }
    return &users[0], nil
}
```

---

## Example SQL Output

`Create()` — Postgres:

```sql
INSERT INTO users (name, email, role, active, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id
```

`Update()`:

```sql
UPDATE users
SET name = $1, email = $2, updated_at = $3
WHERE id = $4
```

`Delete()` with soft delete:

```sql
UPDATE users
SET deleted_at = $1
WHERE id = $2
```

---

## FAQ

**Is this an ORM?**
No. It only provides simple CRUD helpers. Relationships, joins, and complex queries are your responsibility.

**Does it support relations?**
No. Handle relationships manually with raw SQL or repository methods.

**Can I still write custom SQL?**
Yes. `crud.Executor` is just `*sql.DB` or `*sql.Tx` — use them directly for anything beyond CRUD.

**What happens if a model field has no `db` tag?**
The field name is converted to snake_case and used as the column name.

**What if I set a value explicitly and also have a `default:` tag?**
Explicit values always win. `default:` is only applied if the field is a zero value.

---

## Contributing

Contributions are welcome.
Feel free to open issues or pull requests.

---

## License

MIT