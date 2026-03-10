package crud

import (
	"errors"
	"testing"
)

// ── parseColumn ───────────────────────────────────────────────────────────────

func TestParseColumn(t *testing.T) {
	tests := []struct {
		tag  string
		want string
	}{
		{"", ""},
		{"id", "id"},
		{"id,primary_key", "id"},
		{"email,notnull,unique,index", "email"},
		{"created_at,default:current_timestamp", "created_at"},
		{"role,notnull,default:operator", "role"},
	}

	for _, tt := range tests {
		got := parseColumn(tt.tag)
		if got != tt.want {
			t.Errorf("parseColumn(%q) = %q, want %q", tt.tag, got, tt.want)
		}
	}
}

// ── isPrimaryKey ──────────────────────────────────────────────────────────────

func TestIsPrimaryKey(t *testing.T) {
	tests := []struct {
		tag  string
		want bool
	}{
		{"", false},
		{"id", false},
		{"id,primary_key", true},
		{"email,notnull,unique", false},
		{"user_id,primary_key,notnull", true},
	}

	for _, tt := range tests {
		got := isPrimaryKey(tt.tag)
		if got != tt.want {
			t.Errorf("isPrimaryKey(%q) = %v, want %v", tt.tag, got, tt.want)
		}
	}
}

// ── toSnakeCase ───────────────────────────────────────────────────────────────

func TestToSnakeCase(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"ID", "id"},
		{"Name", "name"},
		{"UserID", "user_id"},
		{"FirstName", "first_name"},
		{"CreatedAt", "created_at"},
		{"UpdatedAt", "updated_at"},
		{"DeletedAt", "deleted_at"},
		{"HTMLParser", "html_parser"},
		{"MyURLPath", "my_url_path"},
	}

	for _, tt := range tests {
		got := toSnakeCase(tt.input)
		if got != tt.want {
			t.Errorf("toSnakeCase(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// ── getTableName ──────────────────────────────────────────────────────────────

func TestGetTableName_WithTableNamer(t *testing.T) {
	got, err := getTableName(&User{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "users" {
		t.Errorf("got %q, want %q", got, "users")
	}
}

func TestGetTableName_WithoutTableNamer(t *testing.T) {
	_, err := getTableName(&NoTableModel{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrNoTableName) {
		t.Errorf("expected ErrNoTableName, got: %v", err)
	}
}

func TestGetTableName_ValueReceiver(t *testing.T) {
	// TableName on value receiver should also work when passed as pointer
	got, err := getTableName(&NoTagModel{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "no_tag_models" {
		t.Errorf("got %q, want %q", got, "no_tag_models")
	}
}
