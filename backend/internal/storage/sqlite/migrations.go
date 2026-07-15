package sqlite

import (
	"context"
	"fmt"
	"time"
)

const initialMigrationVersion = 1

var initialMigrationStatements = []string{
	`
	CREATE TABLE chats (
		id TEXT PRIMARY KEY,
		title TEXT NOT NULL,
		last_message TEXT NOT NULL DEFAULT '',
		unread_count INTEGER NOT NULL DEFAULT 0
			CHECK (unread_count >= 0)
	)
	`,
	`
	CREATE TABLE messages (
		id TEXT PRIMARY KEY,
		chat_id TEXT NOT NULL,
		author TEXT NOT NULL,
		text TEXT NOT NULL,
		created_at TEXT NOT NULL,

		FOREIGN KEY (chat_id)
			REFERENCES chats(id)
			ON DELETE CASCADE
	)
	`,
	`
	CREATE INDEX idx_messages_chat_id_created_at
	ON messages(chat_id, created_at)
	`,
}

func (s *Storage) applyMigrations(ctx context.Context) error {
	if _, err := s.db.ExecContext(
		ctx,
		`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at TEXT NOT NULL
		)
		`,
	); err != nil {
		return fmt.Errorf("create schema_migrations table: %w", err)
	}

	var applied int

	err := s.db.QueryRowContext(
		ctx,
		`
		SELECT COUNT(*)
		FROM schema_migrations
		WHERE version = ?
		`,
		initialMigrationVersion,
	).Scan(&applied)
	if err != nil {
		return fmt.Errorf("check migration version %d: %w", initialMigrationVersion, err)
	}

	if applied > 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration transaction: %w", err)
	}

	defer func() {
		_ = tx.Rollback()
	}()

	for index, statement := range initialMigrationStatements {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf(
				"execute migration %d statement %d: %w",
				initialMigrationVersion,
				index+1,
				err,
			)
		}
	}

	if _, err := tx.ExecContext(
		ctx,
		`
		INSERT INTO schema_migrations (version, applied_at)
		VALUES (?, ?)
		`,
		initialMigrationVersion,
		time.Now().UTC().Format(time.RFC3339),
	); err != nil {
		return fmt.Errorf(
			"record migration version %d: %w",
			initialMigrationVersion,
			err,
		)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf(
			"commit migration version %d: %w",
			initialMigrationVersion,
			err,
		)
	}

	return nil
}
