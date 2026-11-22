package db

import (
	"path/filepath"
	"testing"
)

func TestOpenMigratesAndStoresSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	conn, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := SetSetting(conn, "project_id", "abc"); err != nil {
		t.Fatal(err)
	}
	SetSetting(conn, "project_id", "def")
	if got := GetSetting(conn, "project_id"); got != "def" {
		t.Fatalf("got %q", got)
	}
	conn.Close()

	// Re-opening must not re-run migrations.
	conn, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	var v int
	conn.QueryRow("PRAGMA user_version").Scan(&v)
	if v != 7 {
		t.Fatalf("user_version = %d", v)
	}
}
