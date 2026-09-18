package sqlite

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite"

	"myaukalo/backend/internal/domain"
	"myaukalo/backend/internal/storage"
)

// Проверка на этапе компиляции: *Storage обязан реализовать storage.Storage.
var _ storage.Storage = (*Storage)(nil)

type Storage struct {
	db *sql.DB
}

func New(dsn string) (*Storage, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}

	// SQLite плохо переносит конкурентную запись с нескольких соединений,
	// поэтому ограничиваем пул одним соединением — это спасает от SQLITE_BUSY.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite database: %w", err)
	}

	if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("enable sqlite foreign keys: %w", err)
	}

	store := &Storage{db: db}

	if err := store.applyMigrations(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("apply sqlite migrations: %w", err)
	}

	return store, nil
}

func (s *Storage) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// ---------- Чаты ----------

func (s *Storage) GetChats() ([]domain.Chat, error) {
	rows, err := s.db.Query(`
		SELECT id, title, last_message, unread_count
		FROM chats
		ORDER BY rowid
	`)
	if err != nil {
		return nil, fmt.Errorf("select chats: %w", err)
	}
	defer rows.Close()

	chats := make([]domain.Chat, 0)
	for rows.Next() {
		var chat domain.Chat
		if err := rows.Scan(&chat.ID, &chat.Title, &chat.LastMessage, &chat.UnreadCount); err != nil {
			return nil, fmt.Errorf("scan chat: %w", err)
		}
		chats = append(chats, chat)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate chats: %w", err)
	}

	return chats, nil
}

func (s *Storage) GetChatByID(chatID string) (domain.Chat, error) {
	var chat domain.Chat

	err := s.db.QueryRow(`
		SELECT id, title, last_message, unread_count
		FROM chats
		WHERE id = ?
	`, chatID).Scan(&chat.ID, &chat.Title, &chat.LastMessage, &chat.UnreadCount)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Chat{}, storage.ErrChatNotFound
	}
	if err != nil {
		return domain.Chat{}, fmt.Errorf("select chat %s: %w", chatID, err)
	}

	return chat, nil
}

func (s *Storage) AddChat(title string) (domain.Chat, error) {
	id, err := newID("chat")
	if err != nil {
		return domain.Chat{}, err
	}

	chat := domain.Chat{ID: id, Title: title}

	_, err = s.db.Exec(`
		INSERT INTO chats (id, title, last_message, unread_count)
		VALUES (?, ?, ?, ?)
	`, chat.ID, chat.Title, chat.LastMessage, chat.UnreadCount)
	if err != nil {
		return domain.Chat{}, fmt.Errorf("insert chat: %w", err)
	}

	return chat, nil
}

func (s *Storage) MarkChatAsRead(chatID string) (domain.Chat, error) {
	result, err := s.db.Exec(`
		UPDATE chats
		SET unread_count = 0
		WHERE id = ?
	`, chatID)
	if err != nil {
		return domain.Chat{}, fmt.Errorf("mark chat %s as read: %w", chatID, err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return domain.Chat{}, fmt.Errorf("rows affected for chat %s: %w", chatID, err)
	}
	if affected == 0 {
		return domain.Chat{}, storage.ErrChatNotFound
	}

	return s.GetChatByID(chatID)
}

// ---------- Сообщения ----------

func (s *Storage) GetMessages() ([]domain.Message, error) {
	rows, err := s.db.Query(`
		SELECT id, chat_id, author, text, created_at
		FROM messages
		ORDER BY created_at, rowid
	`)
	if err != nil {
		return nil, fmt.Errorf("select messages: %w", err)
	}
	defer rows.Close()

	return scanMessages(rows)
}

func (s *Storage) GetMessagesByChatID(chatID string) ([]domain.Message, error) {
	rows, err := s.db.Query(`
		SELECT id, chat_id, author, text, created_at
		FROM messages
		WHERE chat_id = ?
		ORDER BY created_at, rowid
	`, chatID)
	if err != nil {
		return nil, fmt.Errorf("select messages for chat %s: %w", chatID, err)
	}
	defer rows.Close()

	return scanMessages(rows)
}

func (s *Storage) AddMessage(chatID string, author string, text string) (domain.Message, error) {
	id, err := newID("msg")
	if err != nil {
		return domain.Message{}, err
	}

	message := domain.Message{
		ID:        id,
		ChatID:    chatID,
		Author:    author,
		Text:      text,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}

	// Транзакция: либо обновился чат И вставилось сообщение, либо ничего.
	tx, err := s.db.Begin()
	if err != nil {
		return domain.Message{}, fmt.Errorf("begin transaction: %w", err)
	}
	// Если ниже случится return до Commit — транзакция откатится.
	// После успешного Commit этот Rollback просто ничего не делает.
	defer func() { _ = tx.Rollback() }()

	result, err := tx.Exec(`
		UPDATE chats
		SET last_message = ?, unread_count = unread_count + 1
		WHERE id = ?
	`, message.Text, chatID)
	if err != nil {
		return domain.Message{}, fmt.Errorf("update chat last message: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return domain.Message{}, fmt.Errorf("rows affected: %w", err)
	}
	if affected == 0 {
		return domain.Message{}, storage.ErrChatNotFound
	}

	_, err = tx.Exec(`
		INSERT INTO messages (id, chat_id, author, text, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, message.ID, message.ChatID, message.Author, message.Text, message.CreatedAt)
	if err != nil {
		return domain.Message{}, fmt.Errorf("insert message: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return domain.Message{}, fmt.Errorf("commit transaction: %w", err)
	}

	return message, nil
}

// ---------- Вспомогательные ----------

// scanMessages читает все строки результата запроса в слайс сообщений.
func scanMessages(rows *sql.Rows) ([]domain.Message, error) {
	messages := make([]domain.Message, 0)

	for rows.Next() {
		var message domain.Message
		if err := rows.Scan(
			&message.ID,
			&message.ChatID,
			&message.Author,
			&message.Text,
			&message.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		messages = append(messages, message)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate messages: %w", err)
	}

	return messages, nil
}

// newID генерирует идентификатор вида "chat-<32 hex>" или "msg-<32 hex>".
// Используем crypto/rand, а не счётчик: ID непредсказуемы, их нельзя
// угадать или перебрать — это важно для мессенджера с упором на анонимность.
func newID(prefix string) (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate id: %w", err)
	}
	return fmt.Sprintf("%s-%x", prefix, buf), nil
}
