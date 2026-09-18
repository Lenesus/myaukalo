package storage

import (
	"errors"

	"myaukalo/backend/internal/domain"
)

// ErrChatNotFound возвращается, когда запрашиваемый чат не найден в хранилище.
var ErrChatNotFound = errors.New("chat not found")

// Storage описывает контракт хранилища данных мессенджера.
// Любая реализация (memory, sqlite и т.д.) обязана реализовать все методы.
type Storage interface {
	GetChats() ([]domain.Chat, error)
	GetChatByID(chatID string) (domain.Chat, error)
	AddChat(title string) (domain.Chat, error)
	MarkChatAsRead(chatID string) (domain.Chat, error)

	GetMessages() ([]domain.Message, error)
	GetMessagesByChatID(chatID string) ([]domain.Message, error)
	AddMessage(chatID string, author string, text string) (domain.Message, error)
}
