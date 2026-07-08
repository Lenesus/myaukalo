package memory

import (
	"fmt"
	"sync"
	"time"

	"myaukalo/backend/internal/domain"
)

type MemoryStorage struct {
	mu       sync.RWMutex
	chats    []domain.Chat
	messages []domain.Message
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
	}
}

func (s *MemoryStorage) GetChats() []domain.Chat {
	s.mu.RLock()
	defer s.mu.RUnlock()

	chats := make([]domain.Chat, len(s.chats))
	copy(chats, s.chats)

	return chats
}

func (s *MemoryStorage) GetChatByID(chatID string) (domain.Chat, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, chat := range s.chats {
		if chat.ID == chatID {
			return chat, true
		}
	}

	return domain.Chat{}, false
}

func (s *MemoryStorage) MarkChatAsRead(chatID string) (domain.Chat, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for index, chat := range s.chats {
		if chat.ID == chatID {
			s.chats[index].UnreadCount = 0
			return s.chats[index], true
		}
	}

	return domain.Chat{}, false
}

func (s *MemoryStorage) AddChat(title string) domain.Chat {
	s.mu.Lock()
	defer s.mu.Unlock()

	chat := domain.Chat{
		ID:          fmt.Sprintf("chat-%d", len(s.chats)+1),
		Title:       title,
		LastMessage: "",
		UnreadCount: 0,
	}

	s.chats = append(s.chats, chat)

	return chat
}

func (s *MemoryStorage) MarkChatAsRead(chatID string) (domain.Chat, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for index, chat := range s.chats {
		if chat.ID == chatID {
			s.chats[index].UnreadCount = 0
			return s.chats[index], true
		}
	}

	return domain.Chat{}, false
}

func (s *MemoryStorage) GetMessages() []domain.Message {
	s.mu.RLock()
	defer s.mu.RUnlock()

	messages := make([]domain.Message, len(s.messages))
	copy(messages, s.messages)

	return messages
}

func (s *MemoryStorage) GetMessagesByChatID(chatID string) []domain.Message {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]domain.Message, 0)

	for _, message := range s.messages {
		if message.ChatID == chatID {
			result = append(result, message)
		}
	}

	return result
}

func (s *MemoryStorage) AddMessage(chatID string, author string, text string) (domain.Message, error) {
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
		return domain.Message{}, fmt.Errorf("chat not found")
	}

	message := domain.Message{
		ID:        fmt.Sprintf("msg-%d", len(s.messages)+1),
		ChatID:    chatID,
		Author:    author,
		Text:      text,
		CreatedAt: time.Now().Format(time.RFC3339),
	}

	s.messages = append(s.messages, message)
	s.chats[chatIndex].LastMessage = text
	s.chats[chatIndex].UnreadCount++

	return message, nil
}
