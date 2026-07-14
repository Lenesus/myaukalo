package storage

import "myaukalo/backend/internal/domain"

type Storage interface {
	GetChats() []domain.Chat
	GetChatByID(chatID string) (domain.Chat, bool)
	AddChat(title string) domain.Chat
	MarkChatAsRead(chatID string) (domain.Chat, bool)

	GetMessages() []domain.Message
	GetMessagesByChatID(chatID string) []domain.Message
	AddMessage(chatID string, author string, text string) (domain.Message, error)
}
