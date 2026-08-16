package crud

import (
	"reflect"
	"testing"
)

func TestBuildMeta_PrimaryKeyFromTag(t *testing.T) {
	meta := buildMeta(reflect.TypeOf(User{}))

	if meta.primaryKey == nil {
		t.Fatal("expected primaryKey to be set")
	}
	if meta.primaryKey.column != "id" {
		t.Errorf("primaryKey.column = %q, want %q", meta.primaryKey.column, "id")
	}
	if !meta.primaryKey.primaryKey {
		t.Error("primaryKey.primaryKey should be true")
	}
}

func TestBuildMeta_FieldColumns(t *testing.T) {
	meta := buildMeta(reflect.TypeOf(User{}))

	colSet := make(map[string]bool)
	for _, f := range meta.fields {
		colSet[f.column] = true
	}

	expected := []string{"id", "name", "email", "active", "deleted_at", "created_at", "updated_at"}
	for _, col := range expected {
		if !colSet[col] {
			t.Errorf("expected column %q to be present in meta.fields", col)
		}
	}
}

func TestBuildMeta_PrimaryKeyNotIncludedInNonPKFields(t *testing.T) {
	meta := buildMeta(reflect.TypeOf(User{}))

	for _, f := range meta.fields {
		if f.column == "id" && !f.primaryKey {
			t.Error("id field should be marked as primaryKey")
		}
	}
}

func TestBuildMeta_SnakeCaseFallback(t *testing.T) {
	meta := buildMeta(reflect.TypeOf(NoTagModel{}))

	colSet := make(map[string]bool)
	for _, f := range meta.fields {
		colSet[f.column] = true
	}

	expected := []string{"id", "first_name", "user_id", "created_at"}
	for _, col := range expected {
		if !colSet[col] {
			t.Errorf("expected snake_case column %q, not found in meta", col)
		}
	}
}

func TestBuildMeta_FallbackPrimaryKey(t *testing.T) {
	// NoTagModel has no primary_key tag but has a field named "ID" → "id"
	meta := buildMeta(reflect.TypeOf(NoTagModel{}))

	if meta.primaryKey == nil {
		t.Fatal("expected fallback primaryKey to be set for field named ID")
	}
	if meta.primaryKey.column != "id" {
		t.Errorf("fallback primaryKey.column = %q, want %q", meta.primaryKey.column, "id")
	}
}

func TestGetMeta_Caching(t *testing.T) {
	// clear cache for this type first
	metaCache.Delete(reflect.TypeOf(User{}))

	m1 := getMeta(&User{})
	m2 := getMeta(&User{})

	// should be the exact same pointer from cache
	if m1 != m2 {
		t.Error("getMeta should return cached pointer on second call")
	}
}
