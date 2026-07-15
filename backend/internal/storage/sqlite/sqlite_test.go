package sqlite

import (
	"context"
	"testing"
)

func TestNewCreatesDatabaseSchema(t *testing.T) {
	store, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create sqlite storage: %v", err)
	}

	defer func() {
		if err := store.Close(); err != nil {
			t.Errorf("failed to close sqlite storage: %v", err)
		}
	}()

	expectedTables := []string{
		"schema_migrations",
		"chats",
		"messages",
	}

	for _, expectedTable := range expectedTables {
		var actualTable string

		err := store.db.QueryRow(
			`
			SELECT name
			FROM sqlite_master
			WHERE type = 'table'
			  AND name = ?
			`,
			expectedTable,
		).Scan(&actualTable)
		if err != nil {
			t.Fatalf(
				"expected table %q to exist: %v",
				expectedTable,
				err,
			)
		}

		if actualTable != expectedTable {
			t.Errorf(
				"expected table %q, got %q",
				expectedTable,
				actualTable,
			)
		}
	}
}

func TestInitialMigrationIsIdempotent(t *testing.T) {
	store, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create sqlite storage: %v", err)
	}

	defer func() {
		if err := store.Close(); err != nil {
			t.Errorf("failed to close sqlite storage: %v", err)
		}
	}()

	if err := store.applyMigrations(context.Background()); err != nil {
		t.Fatalf("failed to apply migrations for the second time: %v", err)
	}

	var migrationCount int

	err = store.db.QueryRow(
		`
		SELECT COUNT(*)
		FROM schema_migrations
		WHERE version = ?
		`,
		initialMigrationVersion,
	).Scan(&migrationCount)
	if err != nil {
		t.Fatalf("failed to count migration records: %v", err)
	}

	if migrationCount != 1 {
		t.Errorf(
			"expected one migration record, got %d",
			migrationCount,
		)
	}
}

func TestForeignKeysAreEnabled(t *testing.T) {
	store, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create sqlite storage: %v", err)
	}

	defer func() {
		if err := store.Close(); err != nil {
			t.Errorf("failed to close sqlite storage: %v", err)
		}
	}()

	var foreignKeysEnabled int

	if err := store.db.QueryRow("PRAGMA foreign_keys").Scan(&foreignKeysEnabled); err != nil {
		t.Fatalf("failed to read foreign_keys pragma: %v", err)
	}

	if foreignKeysEnabled != 1 {
		t.Errorf(
			"expected foreign keys to be enabled, got %d",
			foreignKeysEnabled,
		)
	}
}
