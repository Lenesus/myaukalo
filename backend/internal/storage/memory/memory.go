package memory

import (
	"fmt"
	"myaukalo/backend/internal/domain"
	"time"
)

type MemoryStorage struct {
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
	return s.chats
}

func (s *MemoryStorage) GetChatByID(chatID string) (domain.Chat, bool) {
	for _, chat := range s.chats {
		if chat.ID == chatID {
			return chat, true
		}
	}

	return domain.Chat{}, false
}

func (s *MemoryStorage) AddChat(title string) domain.Chat {
	chat := domain.Chat{
		ID:          fmt.Sprintf("chat-%d", len(s.chats)+1),
		Title:       title,
		LastMessage: "",
		UnreadCount: 0,
	}

	s.chats = append(s.chats, chat)

	return chat
}

func (s *MemoryStorage) GetMessages() []domain.Message {
	return s.messages
}

func (s *MemoryStorage) GetMessagesByChatID(chatID string) []domain.Message {
	result := make([]domain.Message, 0)

	for _, message := range s.messages {
		if message.ChatID == chatID {
			result = append(result, message)
		}
	}

	return result
}

func (s *MemoryStorage) AddMessage(chatID string, author string, text string) (domain.Message, error) {

	chatIndex := -1
	for i, chat := range s.chats {
		if chat.ID == chatID {
			chatIndex = i
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

	return message, nil

}
