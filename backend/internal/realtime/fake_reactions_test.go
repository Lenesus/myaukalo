package realtime

import (
	"myaukalo/backend/internal/domain"
	"myaukalo/backend/internal/storage"
)

// Заглушки реакций для тестового fakeStorage из sse_test.go.
// SSE-тесты реакции не вызывают; методы нужны только чтобы fake
// удовлетворял расширенному интерфейсу storage.Storage.
// Вынесены в отдельный файл, чтобы не трогать sse_test.go.

func (f *fakeStorage) AddReaction(messageID string, user string, emoji string) (domain.Message, error) {
	return domain.Message{}, storage.ErrMessageNotFound
}

func (f *fakeStorage) RemoveReaction(messageID string, user string, emoji string) (domain.Message, error) {
	return domain.Message{}, storage.ErrMessageNotFound
}

func (f *fakeStorage) NotifyTyping(chatID string, user string) error {
	return nil
}
