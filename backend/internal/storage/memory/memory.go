package memory

import (
	"fmt"
	"sync"
	"time"

	"myaukalo/backend/internal/domain"
	storagecontract "myaukalo/backend/internal/storage"
)

type MemoryStorage struct {
	mu            sync.RWMutex
	chats         []domain.Chat
	messages      []domain.Message
	nextChatID    int
	nextMessageID int
}

func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		chats: []domain.Chat{
			{
				ID:          "chat-1",
				Title:       "Котокафе",
				LastMessage: "мяу, кто тут?",
				UnreadCount: 2,
			},
			{
				ID:          "chat-2",
				Title:       "Лапки и хвостики",
				LastMessage: "пора кормить котиков",
				UnreadCount: 0,
			},
		},
		messages: []domain.Message{
			{
				ID:        "msg-1",
				ChatID:    "chat-1",
				Author:    "Барсик",
				Text:      "мяу, кто тут?",
				CreatedAt: "2026-05-20T13:00:00Z",
			},
			{
				ID:        "msg-2",
				ChatID:    "chat-1",
				Author:    "Мурка",
				Text:      "я тут, принесла клубок",
				CreatedAt: "2026-05-20T13:01:00Z",
			},
			{
				ID:        "msg-3",
				ChatID:    "chat-2",
				Author:    "Рыжик",
				Text:      "пора кормить котиков",
				CreatedAt: "2026-05-20T13:02:00Z",
			},
		},
		nextChatID:    3,
		nextMessageID: 4,
	}
}

var _ storagecontract.Storage = (*MemoryStorage)(nil)

func (s *MemoryStorage) GetChats() ([]domain.Chat, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	chats := make([]domain.Chat, len(s.chats))
	copy(chats, s.chats)

	return chats, nil
}

func (s *MemoryStorage) GetChatByID(chatID string) (domain.Chat, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, chat := range s.chats {
		if chat.ID == chatID {
			return chat, nil
		}
	}

	return domain.Chat{}, storagecontract.ErrChatNotFound
}

func (s *MemoryStorage) AddChat(title string) (domain.Chat, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	chat := domain.Chat{
		ID:          fmt.Sprintf("chat-%d", s.nextChatID),
		Title:       title,
		LastMessage: "",
		UnreadCount: 0,
	}

	s.nextChatID++
	s.chats = append(s.chats, chat)

	return chat, nil
}

func (s *MemoryStorage) MarkChatAsRead(chatID string) (domain.Chat, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for index, chat := range s.chats {
		if chat.ID == chatID {
			s.chats[index].UnreadCount = 0
			return s.chats[index], nil
		}
	}

	return domain.Chat{}, storagecontract.ErrChatNotFound
}

func (s *MemoryStorage) GetMessages() ([]domain.Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now()
	out := make([]domain.Message, 0, len(s.messages))

	for _, message := range s.messages {
		if isExpired(message, now) {
			continue
		}
		out = append(out, message)
	}

	return out, nil
}

func (s *MemoryStorage) GetMessagesByChatID(chatID string) ([]domain.Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now()
	out := make([]domain.Message, 0)

	for _, message := range s.messages {
		if message.ChatID != chatID || isExpired(message, now) {
			continue
		}
		out = append(out, message)
	}

	return out, nil
}

func (s *MemoryStorage) AddMessage(chatID string, author string, text string, ttl time.Duration) (domain.Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	chatIndex := -1
	for index, chat := range s.chats {
		if chat.ID == chatID {
			chatIndex = index
			break
		}
	}

	if chatIndex == -1 {
		return domain.Message{}, storagecontract.ErrChatNotFound
	}

	now := time.Now().UTC()
	message := domain.Message{
		ID:        fmt.Sprintf("msg-%d", s.nextMessageID),
		ChatID:    chatID,
		Author:    author,
		Text:      text,
		CreatedAt: now.Format(time.RFC3339),
	}

	if ttl > 0 {
		expiresAt := now.Add(ttl).Format(time.RFC3339)
		message.ExpiresAt = &expiresAt
	}

	s.nextMessageID++
	s.messages = append(s.messages, message)
	s.chats[chatIndex].LastMessage = text
	s.chats[chatIndex].UnreadCount++

	return message, nil
}

func (s *MemoryStorage) DeleteExpiredMessages(now time.Time) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	kept := make([]domain.Message, 0, len(s.messages))
	removed := 0

	for _, message := range s.messages {
		if isExpired(message, now) {
			removed++
			continue
		}
		kept = append(kept, message)
	}

	s.messages = kept

	return removed, nil
}

func isExpired(message domain.Message, now time.Time) bool {
	if message.ExpiresAt == nil {
		return false
	}

	expiresAt, err := time.Parse(time.RFC3339, *message.ExpiresAt)
	if err != nil {
		return true
	}

	return !expiresAt.After(now)
}
