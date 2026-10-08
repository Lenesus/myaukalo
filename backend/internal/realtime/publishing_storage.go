package realtime

import (
	"time"

	"myaukalo/backend/internal/domain"
	"myaukalo/backend/internal/storage"
)

type PublishingStorage struct {
	inner storage.Storage
	hub   *Hub
}

var _ storage.Storage = (*PublishingStorage)(nil)

func NewPublishingStorage(inner storage.Storage, hub *Hub) *PublishingStorage {
	return &PublishingStorage{inner: inner, hub: hub}
}

func (s *PublishingStorage) GetChats() ([]domain.Chat, error) {
	return s.inner.GetChats()
}

func (s *PublishingStorage) GetChatByID(chatID string) (domain.Chat, error) {
	return s.inner.GetChatByID(chatID)
}

func (s *PublishingStorage) AddChat(title string) (domain.Chat, error) {
	return s.inner.AddChat(title)
}

func (s *PublishingStorage) MarkChatAsRead(chatID string) (domain.Chat, error) {
	return s.inner.MarkChatAsRead(chatID)
}

func (s *PublishingStorage) GetMessages() ([]domain.Message, error) {
	return s.inner.GetMessages()
}

func (s *PublishingStorage) GetMessagesByChatID(chatID string) ([]domain.Message, error) {
	return s.inner.GetMessagesByChatID(chatID)
}

func (s *PublishingStorage) AddMessage(chatID string, author string, text string, ttl time.Duration) (domain.Message, error) {
	message, err := s.inner.AddMessage(chatID, author, text, ttl)
	if err != nil {
		return message, err
	}

	s.hub.Publish(message)

	return message, nil
}
