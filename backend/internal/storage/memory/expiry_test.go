package memory

import (
	"testing"
	"time"

	"myaukalo/backend/internal/domain"
)

func TestMemory_AddMessageWithTTL(t *testing.T) {
	store := NewMemoryStorage()

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
}

func TestMemory_GetMessagesFiltersExpired(t *testing.T) {
	store := NewMemoryStorage()

	chat, err := store.AddChat("TTL-чат")
	if err != nil {
		t.Fatalf("AddChat: %v", err)
	}

	if _, err := store.AddMessage(chat.ID, "Барсик", "живое", time.Hour); err != nil {
		t.Fatalf("AddMessage: %v", err)
	}

	past := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)

	store.mu.Lock()
	store.messages = append(store.messages, domain.Message{
		ID:        "dead-1",
		ChatID:    chat.ID,
		Author:    "Рыжик",
		Text:      "сгорело",
		CreatedAt: past,
		ExpiresAt: &past,
	})
	store.mu.Unlock()

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

func TestMemory_DeleteExpiredMessages(t *testing.T) {
	store := NewMemoryStorage()

	chat, err := store.AddChat("TTL-чат")
	if err != nil {
		t.Fatalf("AddChat: %v", err)
	}

	if _, err := store.AddMessage(chat.ID, "Барсик", "живое", time.Hour); err != nil {
		t.Fatalf("AddMessage: %v", err)
	}

	past := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)

	store.mu.Lock()
	store.messages = append(store.messages, domain.Message{
		ID:        "dead-1",
		ChatID:    chat.ID,
		Author:    "Рыжик",
		Text:      "сгорело",
		CreatedAt: past,
		ExpiresAt: &past,
	})
	store.mu.Unlock()

	removed, err := store.DeleteExpiredMessages(time.Now().UTC())
	if err != nil {
		t.Fatalf("DeleteExpiredMessages: %v", err)
	}

	if removed != 1 {
		t.Fatalf("expected 1 removed message, got %d", removed)
	}

	messages, err := store.GetMessagesByChatID(chat.ID)
	if err != nil {
		t.Fatalf("GetMessagesByChatID: %v", err)
	}

	if len(messages) != 1 {
		t.Fatalf("expected 1 remaining message, got %d", len(messages))
	}
}
