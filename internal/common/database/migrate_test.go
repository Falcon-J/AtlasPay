package database

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestMigrationFilesAreSelectedAndSortedByVersionedName(t *testing.T) {
	dir := t.TempDir()
	for name, contents := range map[string]string{
		"003_future.sql":  "SELECT 3;",
		"001_initial.sql": "SELECT 1;",
		"002_feature.sql": "SELECT 2;",
		"README.md":       "not a migration",
		"bad.sql":         "not a migration",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	files, err := migrationFiles(dir)
	if err != nil {
		t.Fatalf("migrationFiles returned error: %v", err)
	}

	got := make([]string, len(files))
	for i, file := range files {
		got[i] = file.name
	}
	want := []string{"001_initial.sql", "002_feature.sql", "003_future.sql"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("migration names = %v, want %v", got, want)
	}
}

func TestMigrationFilesRejectDuplicateVersionedNames(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"001_initial.sql", "001_other.sql"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("SELECT 1;"), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	if _, err := migrationFiles(dir); err == nil {
		t.Fatal("migrationFiles accepted duplicate migration versions")
	}
}

func TestMigrationFilesSortByNumericVersion(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"099_later.sql", "0100_latest.sql", "003_initial.sql"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("SELECT 1;"), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	files, err := migrationFiles(dir)
	if err != nil {
		t.Fatalf("migrationFiles returned error: %v", err)
	}

	got := make([]string, len(files))
	for i, file := range files {
		got[i] = file.name
	}
	want := []string{"003_initial.sql", "099_later.sql", "0100_latest.sql"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("migration names = %v, want %v", got, want)
	}
}

func TestMigrationFilesRejectEquivalentNumericVersions(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"099_later.sql", "0099_duplicate.sql"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("SELECT 1;"), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	if _, err := migrationFiles(dir); err == nil {
		t.Fatal("migrationFiles accepted equivalent numeric migration versions")
	}
}
