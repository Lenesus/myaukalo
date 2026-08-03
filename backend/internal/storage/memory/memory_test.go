package memory

import "testing"

func TestAddChatGeneratesSequentialIDs(t *testing.T) {
	store := NewMemoryStorage()

	firstChat, err := store.AddChat("Первый новый чат")
	if err != nil {
		t.Fatalf("failed to add first chat: %v", err)
	}

	secondChat, err := store.AddChat("Второй новый чат")
	if err != nil {
		t.Fatalf("failed to add second chat: %v", err)
	}

	if firstChat.ID != "chat-3" {
		t.Errorf(
			"expected first chat id %q, got %q",
			"chat-3",
			firstChat.ID,
		)
	}

	if secondChat.ID != "chat-4" {
		t.Errorf(
			"expected second chat id %q, got %q",
			"chat-4",
			secondChat.ID,
		)
	}

	if firstChat.ID == secondChat.ID {
		t.Errorf(
			"expected chat ids to be unique, got %q",
			firstChat.ID,
		)
	}
}

func TestAddMessageGeneratesSequentialIDs(t *testing.T) {
	store := NewMemoryStorage()

	firstMessage, err := store.AddMessage(
		"chat-1",
		"Барсик",
		"Первое новое сообщение",
	)
	if err != nil {
		t.Fatalf("failed to add first message: %v", err)
	}

	secondMessage, err := store.AddMessage(
		"chat-1",
		"Мурка",
		"Второе новое сообщение",
	)
	if err != nil {
		t.Fatalf("failed to add second message: %v", err)
	}

	if firstMessage.ID != "msg-4" {
		t.Errorf(
			"expected first message id %q, got %q",
			"msg-4",
			firstMessage.ID,
		)
	}

	if secondMessage.ID != "msg-5" {
		t.Errorf(
			"expected second message id %q, got %q",
			"msg-5",
			secondMessage.ID,
		)
	}

	if firstMessage.ID == secondMessage.ID {
		t.Errorf(
			"expected message ids to be unique, got %q",
			firstMessage.ID,
		)
	}
}
