package storage

import (
	"errors"
	"time"

	"myaukalo/backend/internal/domain"
)

var ErrChatNotFound = errors.New("chat not found")
var ErrMessageNotFound = errors.New("message not found")

type Storage interface {
	GetChats() ([]domain.Chat, error)
	GetChatByID(chatID string) (domain.Chat, error)
	AddChat(title string) (domain.Chat, error)
	MarkChatAsRead(chatID string) (domain.Chat, error)

	GetMessages() ([]domain.Message, error)
	GetMessagesByChatID(chatID string) ([]domain.Message, error)
	AddMessage(chatID string, author string, text string, ttl time.Duration) (domain.Message, error)

	// Реакции возвращают ОБНОВЛЁННОЕ сообщение целиком, чтобы декоратор
	// мог опубликовать его тем же hub.Publish (реакции едут в event: message).
	AddReaction(messageID string, user string, emoji string) (domain.Message, error)
	RemoveReaction(messageID string, user string, emoji string) (domain.Message, error)

	// NotifyTyping — эфемерный сигнал «печатает…», не хранится.
	// Inner-реализации no-op; декоратор рассылает через hub.
	NotifyTyping(chatID string, user string) error
}
