[![Go Reference](https://pkg.go.dev/badge/github.com/ciprianiordache/crud-depot.svg)](https://pkg.go.dev/github.com/ciprianiordache/crud-depot)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

# CRUD Depot

A lightweight and reusable CRUD helper for Go applications.

**CRUD Depot** is a small utility designed to simplify common database operations while staying minimal and predictable.

It is **not an ORM**.
Instead, it provides a thin layer on top of ``database/sql`` to perform common CRUD operations using Go structs.

The goal is to make database access:

- simple
- reusable
- predictable
- portable across multiple projects

## How to get

```bash
go get -u github.com/ciprianiordache/crud-depot
```

## Why This Library Exists

Most Go projects eventually end up repeating the same logic:

- mapping struct fields to SQL columns
- building ``INSERT``
- building ``UPDATE``
- scanning rows
- handling transactions
- managing timestamps
- implementing soft deletes

This library removes that repetitive boilerplate while **still letting you control your SQL architecture**.

## Features 

- Simple CRUD operations
- Supports *sql.DB and *sql.Tx
- Multiple SQL dialects
- Struct → column mapping
- Metadata caching for performance
- Automatic timestamps (optional)
- Soft delete support (optional)
- Transaction helpers
- Small and predictable API
- No ORM complexity

## Supports 

- Postgres
- MySQL
- SQLite

---

# Usage

## Define models

```go

type User struct {
    ID int `db:"id,primary_key"`
    Email string `db:"email"`
    Name string `db:"name"`
}

func (User) TableName() string {
    return "users"
}

```

---

## Create CRUD instance

Postgres:
```go
db, _ := sql.Open("postgres", dsn)
depot := crud.New(db, crud.Postgres{})
```

MySQL: 
```go
db, _ := sql.Open("mysql", dsn)
depot := crud.New(db, crud.MySQL{})
```

SQLite:
```go
db, _ := sql.Open("sqlite3", "./test.db")
depot := crud.New(db, crud.SQLite{})
```

---

## Create 

```go
user := &User{
    Name: "Jhon",
    Email: "jhon@email.com",
}

id, err := depot.Create(user)
```

---

## Read

```go 
var users []User 
err := depot.Read(
    User{},
    "email",
    "john@email.com",
    &users,
)
```

---

## Get (pagination)

```go 
var users []User 
err := depot.Get( 
    User{},
    &users,
    0,
    10,
)
```

---

## Update

```go
user.Name = "John Updated" 
err := depot.Update( 
    user,
    "id",
    user.ID,
)
```

---

## Delete 

```go
err := depot.Delete(
    User{},
    "id",
    user.ID,
)
```

---

## Transactions

The package can run operations inside a transaction.

```go
err := depot.RunInTx(db, func(tx *crud.CRUD) error {
    _, err := tx.Create(&User{
        Name: "Alice",
    })
    if err != nil {
        return err
    }
    return nil
})
```

If the function returns an error:

- the transaction **rolls back**

If it returns ``nil``:

- the transaction **commits**

---

## Optional Interfaces

The package provides several optional interfaces to extend behavior.

### TableNamer (required)

Defines the table name.

```go
func (User) TableName() string {
    return "users"
}
```

---

### Timestamped

Automatically sets ``created_at`` and ``updated_at``.

```go
type User struct {
    CreatedAt time.Time
    UpdatedAt time.Time
}

func (u *User) SetCreatedAt(t time.Time) {
    u.CreatedAt = t
}

func (u *User) SetUpdatedAt(t time.Time) {
    u.UpdatedAt = t
}
```

Behavior:

- ``Create()`` sets ``CreatedAt`` and ``UpdatedAt``
- ``Update()`` updates ``UpdatedAt``

---

### Soft Delete

Instead of deleting a row, the record is marked as deleted.

```go
func (User) SoftDeleteField() string {
    return "deleted_at"
}
```

Delete becomes:

```sql
UPDATE users 
SET deleted_at = NOW()
WHERE id = ?
```

---

## Struct mapping

Columns are resolved from the ``db`` struct tag.

Example: 

```go
Email string `db:"email"`
```

If no tag is provided, the field name is covered to **snake_case**.

Example: 

`CreatedAt -> created_at`
`UserID -> user_id`

---

## Transactions

You can attach an existing transaction:

```go
tx, err := db.Begin()

crudTx := depot.WithTx(tx)

_, err := crudTx.Create(&User{
    Name: "Alice",
})

tx.Commit()
```

---

## Performance

To minimize reflection overhead:

- model metadata is cached
- reflection is executed only once per model type

This allows CRUD operations to remain lightweight.

---

## Recommended usage / best practices 

- Keep complex queries inside repositories.
- Use this tool only for **simple CRUD operations**.
- Combine it with a **repository layer**.
- Use ``RunInTx`` when performing multiple writes.

Exemple repository:

```go
type UserRepository struct {
    depot *crud.CRUD
}

func (r *UserRepository) Create(user *User) (string, error) {
    return r.depot.Create(user)
}
```

---

## Example SQL output

Example ``Create()`` query:

```sql
INSERT INTO users (name, email) 
VALUES ($1, $2) 
RETURNING id
```

Example ``Update()`` query:

```sql
UPDATE users 
SET name = $1, email = $2 
WHERE id = $3
```

---

## FAQ/Gotchas

- Q: Is this an ORM?
    - A: No, it only provides simple CRUD helpers.

- Q: Does it support relations?
   - A: No, relationships should be handled manually.

- Q: Can I still write custom SQL queries?
   - A: Yes. This library only helps with basic CRUD operations.

---

## Contributing   

Contributions are welcome!
Feel free to open issues or pull requests.

---

## License

MIT