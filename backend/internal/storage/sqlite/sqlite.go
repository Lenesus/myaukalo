package sqlite

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"myaukalo/backend/internal/domain"
	"myaukalo/backend/internal/storage"
)

var _ storage.Storage = (*Storage)(nil)

type Storage struct {
	db *sql.DB
}

func New(dsn string) (*Storage, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}

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

	chat := domain.Chat{
		ID:    id,
		Title: title,
	}

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

func (s *Storage) GetMessages() ([]domain.Message, error) {
	now := time.Now().UTC().Format(time.RFC3339)

	rows, err := s.db.Query(`
		SELECT id, chat_id, author, text, created_at, expires_at
		FROM messages
		WHERE expires_at IS NULL OR expires_at > ?
		ORDER BY created_at, rowid
	`, now)
	if err != nil {
		return nil, fmt.Errorf("select messages: %w", err)
	}

	messages, err := scanMessages(rows)
	// ВАЖНО: закрываем rows ДО второго запроса за реакциями —
	// SetMaxOpenConns(1), иначе deadlock на одном соединении.
	_ = rows.Close()
	if err != nil {
		return nil, err
	}

	if err := s.attachReactions(messages); err != nil {
		return nil, err
	}

	return messages, nil
}

func (s *Storage) GetMessagesByChatID(chatID string) ([]domain.Message, error) {
	now := time.Now().UTC().Format(time.RFC3339)

	rows, err := s.db.Query(`
		SELECT id, chat_id, author, text, created_at, expires_at
		FROM messages
		WHERE chat_id = ? AND (expires_at IS NULL OR expires_at > ?)
		ORDER BY created_at, rowid
	`, chatID, now)
	if err != nil {
		return nil, fmt.Errorf("select messages for chat %s: %w", chatID, err)
	}

	messages, err := scanMessages(rows)
	_ = rows.Close()
	if err != nil {
		return nil, err
	}

	if err := s.attachReactions(messages); err != nil {
		return nil, err
	}

	return messages, nil
}

func (s *Storage) AddMessage(chatID string, author string, text string, ttl time.Duration) (domain.Message, error) {
	id, err := newID("msg")
	if err != nil {
		return domain.Message{}, err
	}

	now := time.Now().UTC()
	message := domain.Message{
		ID:        id,
		ChatID:    chatID,
		Author:    author,
		Text:      text,
		CreatedAt: now.Format(time.RFC3339),
	}

	if ttl > 0 {
		expiresAt := now.Add(ttl).Format(time.RFC3339)
		message.ExpiresAt = &expiresAt
	}

	tx, err := s.db.Begin()
	if err != nil {
		return domain.Message{}, fmt.Errorf("begin transaction: %w", err)
	}
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

	var expiresAtArg any
	if message.ExpiresAt != nil {
		expiresAtArg = *message.ExpiresAt
	}

	_, err = tx.Exec(`
		INSERT INTO messages (id, chat_id, author, text, created_at, expires_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, message.ID, message.ChatID, message.Author, message.Text, message.CreatedAt, expiresAtArg)
	if err != nil {
		return domain.Message{}, fmt.Errorf("insert message: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return domain.Message{}, fmt.Errorf("commit transaction: %w", err)
	}

	return message, nil
}

func (s *Storage) DeleteExpiredMessages(now time.Time) (int, error) {
	result, err := s.db.Exec(`
		DELETE FROM messages
		WHERE expires_at IS NOT NULL AND expires_at <= ?
	`, now.UTC().Format(time.RFC3339))
	if err != nil {
		return 0, fmt.Errorf("delete expired messages: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("rows affected for expiry delete: %w", err)
	}

	return int(affected), nil
}

// AddReaction идемпотентно добавляет (user,emoji) к сообщению и возвращает
// обновлённое сообщение с агрегированными реакциями.
func (s *Storage) AddReaction(messageID string, user string, emoji string) (domain.Message, error) {
	if _, err := s.messageChatID(messageID); err != nil {
		return domain.Message{}, err
	}

	_, err := s.db.Exec(`
		INSERT OR IGNORE INTO reactions (message_id, user, emoji)
		VALUES (?, ?, ?)
	`, messageID, user, emoji)
	if err != nil {
		return domain.Message{}, fmt.Errorf("insert reaction: %w", err)
	}

	return s.getMessageWithReactions(messageID)
}

// RemoveReaction удаляет (user,emoji) и возвращает обновлённое сообщение.
func (s *Storage) RemoveReaction(messageID string, user string, emoji string) (domain.Message, error) {
	if _, err := s.messageChatID(messageID); err != nil {
		return domain.Message{}, err
	}

	_, err := s.db.Exec(`
		DELETE FROM reactions
		WHERE message_id = ? AND user = ? AND emoji = ?
	`, messageID, user, emoji)
	if err != nil {
		return domain.Message{}, fmt.Errorf("delete reaction: %w", err)
	}

	return s.getMessageWithReactions(messageID)
}

// messageChatID проверяет, что сообщение существует и не протухло.
func (s *Storage) messageChatID(messageID string) (string, error) {
	now := time.Now().UTC().Format(time.RFC3339)

	var chatID string
	err := s.db.QueryRow(`
		SELECT chat_id
		FROM messages
		WHERE id = ? AND (expires_at IS NULL OR expires_at > ?)
	`, messageID, now).Scan(&chatID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", storage.ErrMessageNotFound
	}
	if err != nil {
		return "", fmt.Errorf("select message %s: %w", messageID, err)
	}

	return chatID, nil
}

func (s *Storage) getMessageWithReactions(messageID string) (domain.Message, error) {
	now := time.Now().UTC().Format(time.RFC3339)

	var message domain.Message
	var expiresAt sql.NullString

	err := s.db.QueryRow(`
		SELECT id, chat_id, author, text, created_at, expires_at
		FROM messages
		WHERE id = ? AND (expires_at IS NULL OR expires_at > ?)
	`, messageID, now).Scan(
		&message.ID, &message.ChatID, &message.Author,
		&message.Text, &message.CreatedAt, &expiresAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Message{}, storage.ErrMessageNotFound
	}
	if err != nil {
		return domain.Message{}, fmt.Errorf("select message %s: %w", messageID, err)
	}

	if expiresAt.Valid {
		value := expiresAt.String
		message.ExpiresAt = &value
	}

	messages := []domain.Message{message}
	if err := s.attachReactions(messages); err != nil {
		return domain.Message{}, err
	}

	return messages[0], nil
}

// attachReactions грузит реакции для всех переданных сообщений ОДНИМ запросом
// и агрегирует по эмодзи, сохраняя порядок первого появления.
func (s *Storage) attachReactions(messages []domain.Message) error {
	if len(messages) == 0 {
		return nil
	}

	placeholders := make([]string, len(messages))
	args := make([]any, len(messages))
	for i, m := range messages {
		placeholders[i] = "?"
		args[i] = m.ID
	}

	rows, err := s.db.Query(`
		SELECT message_id, user, emoji
		FROM reactions
		WHERE message_id IN (`+strings.Join(placeholders, ",")+`)
		ORDER BY rowid
	`, args...)
	if err != nil {
		return fmt.Errorf("select reactions: %w", err)
	}
	defer rows.Close()

	usersByEmoji := make(map[string]map[string][]string)
	emojiOrder := make(map[string][]string)

	for rows.Next() {
		var messageID, user, emoji string
		if err := rows.Scan(&messageID, &user, &emoji); err != nil {
			return fmt.Errorf("scan reaction: %w", err)
		}

		byEmoji, ok := usersByEmoji[messageID]
		if !ok {
			byEmoji = make(map[string][]string)
			usersByEmoji[messageID] = byEmoji
		}

		if _, seen := byEmoji[emoji]; !seen {
			emojiOrder[messageID] = append(emojiOrder[messageID], emoji)
		}
		byEmoji[emoji] = append(byEmoji[emoji], user)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate reactions: %w", err)
	}

	for i := range messages {
		order := emojiOrder[messages[i].ID]
		if len(order) == 0 {
			continue
		}

		reactions := make([]domain.Reaction, 0, len(order))
		for _, emoji := range order {
			reactions = append(reactions, domain.Reaction{
				Emoji: emoji,
				Users: usersByEmoji[messages[i].ID][emoji],
			})
		}
		messages[i].Reactions = reactions
	}

	return nil
}

func scanMessages(rows *sql.Rows) ([]domain.Message, error) {
	messages := make([]domain.Message, 0)

	for rows.Next() {
		var message domain.Message
		var expiresAt sql.NullString

		if err := rows.Scan(
			&message.ID,
			&message.ChatID,
			&message.Author,
			&message.Text,
			&message.CreatedAt,
			&expiresAt,
		); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}

		if expiresAt.Valid {
			value := expiresAt.String
			message.ExpiresAt = &value
		}

		messages = append(messages, message)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate messages: %w", err)
	}

	return messages, nil
}

func newID(prefix string) (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate id: %w", err)
	}
	return fmt.Sprintf("%s-%x", prefix, buf), nil
}
