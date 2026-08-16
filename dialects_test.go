package crud

import (
	"testing"
)

func TestPostgresDialect(t *testing.T) {
	d := Postgres{}

	t.Run("Placeholder", func(t *testing.T) {
		tests := []struct{ n int; want string }{
			{1, "$1"},
			{2, "$2"},
			{10, "$10"},
		}
		for _, tt := range tests {
			got := d.Placeholder(tt.n)
			if got != tt.want {
				t.Errorf("Placeholder(%d) = %q, want %q", tt.n, got, tt.want)
			}
		}
	})

	t.Run("ReturningClause", func(t *testing.T) {
		got := d.ReturningClause("id")
		want := "RETURNING id"
		if got != want {
			t.Errorf("ReturningClause(%q) = %q, want %q", "id", got, want)
		}

		got = d.ReturningClause("user_id")
		want = "RETURNING user_id"
		if got != want {
			t.Errorf("ReturningClause(%q) = %q, want %q", "user_id", got, want)
		}
	})

	t.Run("UsesLastInsertID", func(t *testing.T) {
		if d.UsesLastInsertID() {
			t.Error("Postgres.UsesLastInsertID() should be false")
		}
	})
}

func TestMySQLDialect(t *testing.T) {
	d := MySQL{}

	t.Run("Placeholder", func(t *testing.T) {
		// MySQL always returns "?" regardless of n
		for _, n := range []int{1, 2, 5, 100} {
			got := d.Placeholder(n)
			if got != "?" {
				t.Errorf("MySQL.Placeholder(%d) = %q, want %q", n, got, "?")
			}
		}
	})

	t.Run("ReturningClause", func(t *testing.T) {
		got := d.ReturningClause("id")
		if got != "" {
			t.Errorf("MySQL.ReturningClause() = %q, want %q", got, "")
		}
	})

	t.Run("UsesLastInsertID", func(t *testing.T) {
		if !d.UsesLastInsertID() {
			t.Error("MySQL.UsesLastInsertID() should be true")
		}
	})
}

func TestSQLiteDialect(t *testing.T) {
	d := SQLite{}

	t.Run("Placeholder", func(t *testing.T) {
		for _, n := range []int{1, 2, 5, 100} {
			got := d.Placeholder(n)
			if got != "?" {
				t.Errorf("SQLite.Placeholder(%d) = %q, want %q", n, got, "?")
			}
		}
	})

	t.Run("ReturningClause", func(t *testing.T) {
		got := d.ReturningClause("id")
		if got != "" {
			t.Errorf("SQLite.ReturningClause() = %q, want %q", got, "")
		}
	})

	t.Run("UsesLastInsertID", func(t *testing.T) {
		if !d.UsesLastInsertID() {
			t.Error("SQLite.UsesLastInsertID() should be true")
		}
	})
}
