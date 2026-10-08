package sqlite

import (
	"path/filepath"
	"testing"
	"time"
)

func TestSQLite_AddMessageWithTTLStoresExpiresAt(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "ttl.db")

	store, err := New(dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = store.Close() }()

	chat, err := store.AddChat("TTL-чат")
	if err != nil {
		t.Fatalf("AddChat: %v", err)
	}

	message, err := store.AddMessage(chat.ID, "Барсик", "сгорит", time.Hour)
	if err != nil {
		t.Fatalf("AddMessage: %v", err)
	}

	if message.ExpiresAt == nil {
		t.Fatal("expected expires_at for ttl > 0")
	}

	messages, err := store.GetMessagesByChatID(chat.ID)
	if err != nil {
		t.Fatalf("GetMessagesByChatID: %v", err)
	}

	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}
	if messages[0].ExpiresAt == nil {
		t.Fatalf("message with expires_at was not returned: %+v", messages[0])
	}
}

func TestSQLite_GetMessagesFiltersExpired(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "ttl.db")

	store, err := New(dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = store.Close() }()

	chat, err := store.AddChat("TTL-чат")
	if err != nil {
		t.Fatalf("AddChat: %v", err)
	}

	if _, err := store.AddMessage(chat.ID, "Барсик", "живое", time.Hour); err != nil {
		t.Fatalf("AddMessage: %v", err)
	}

	createdAt := time.Now().UTC().Add(-2 * time.Hour).Format(time.RFC3339)
	expiresAt := time.Now().UTC().Add(-1 * time.Hour).Format(time.RFC3339)

	if _, err := store.db.Exec(`
		INSERT INTO messages (id, chat_id, author, text, created_at, expires_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, "dead-1", chat.ID, "Рыжик", "сгорело", createdAt, expiresAt); err != nil {
		t.Fatalf("insert expired message: %v", err)
	}

	messages, err := store.GetMessagesByChatID(chat.ID)
	if err != nil {
		t.Fatalf("GetMessagesByChatID: %v", err)
	}

	if len(messages) != 1 {
		t.Fatalf("expected 1 live message, got %d", len(messages))
	}

	for _, message := range messages {
		if message.ID == "dead-1" {
			t.Fatalf("expired message leaked: %+v", message)
		}
	}
}

func TestSQLite_DeleteExpiredMessagesRemovesOnlyExpired(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "ttl.db")

	store, err := New(dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = store.Close() }()

	chat, err := store.AddChat("TTL-чат")
	if err != nil {
		t.Fatalf("AddChat: %v", err)
	}

	if _, err := store.AddMessage(chat.ID, "Барсик", "живое", time.Hour); err != nil {
		t.Fatalf("AddMessage: %v", err)
	}

	createdAt := time.Now().UTC().Add(-2 * time.Hour).Format(time.RFC3339)
	expiresAt := time.Now().UTC().Add(-1 * time.Hour).Format(time.RFC3339)

	if _, err := store.db.Exec(`
		INSERT INTO messages (id, chat_id, author, text, created_at, expires_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, "dead-1", chat.ID, "Рыжик", "сгорело", createdAt, expiresAt); err != nil {
		t.Fatalf("insert expired message: %v", err)
	}

	removed, err := store.DeleteExpiredMessages(time.Now().UTC())
	if err != nil {
		t.Fatalf("DeleteExpiredMessages: %v", err)
	}

	if removed != 1 {
		t.Fatalf("expected 1 removed message, got %d", removed)
	}

	messages, err := store.GetMessages()
	if err != nil {
		t.Fatalf("GetMessages: %v", err)
	}

	if len(messages) != 1 {
		t.Fatalf("expected 1 remaining message, got %d", len(messages))
	}
	if messages[0].Author != "Барсик" || messages[0].Text != "живое" {
		t.Fatalf("unexpected remaining message: %+v", messages[0])
	}
}
