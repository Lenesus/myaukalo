package sqlite

import (
	"path/filepath"
	"testing"
)

// TestPersistence_AcrossReopen — киллер-тест SQLite: данные обязаны
// переживать закрытие и повторное открытие базы. Именно это отличает
// реальное хранилище от in-memory заглушки.
func TestPersistence_AcrossReopen(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "persist.db")

	// Сессия 1: пишем и закрываем соединение.
	store1, err := New(dsn)
	if err != nil {
		t.Fatalf("open store1: %v", err)
	}
	chat, err := store1.AddChat("Переживёт перезапуск")
	if err != nil {
		t.Fatalf("AddChat: %v", err)
	}
	if _, err := store1.AddMessage(chat.ID, "Рыжик", "мяу-мяу"); err != nil {
		t.Fatalf("AddMessage: %v", err)
	}
	if err := store1.Close(); err != nil {
		t.Fatalf("close store1: %v", err)
	}

	// Сессия 2: открываем ТОТ ЖЕ файл заново и читаем.
	store2, err := New(dsn)
	if err != nil {
		t.Fatalf("open store2: %v", err)
	}
	defer func() { _ = store2.Close() }()

	chats, err := store2.GetChats()
	if err != nil {
		t.Fatalf("GetChats: %v", err)
	}
	if len(chats) != 1 || chats[0].ID != chat.ID {
		t.Fatalf("chat was not persisted: %+v", chats)
	}

	msgs, err := store2.GetMessagesByChatID(chat.ID)
	if err != nil {
		t.Fatalf("GetMessagesByChatID: %v", err)
	}
	if len(msgs) != 1 || msgs[0].Text != "мяу-мяу" {
		t.Fatalf("message was not persisted: %+v", msgs)
	}
}
