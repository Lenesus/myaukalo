package memory

// NotifyTyping — no-op: typing эфемерный, в памяти не хранится.
// Рассылку делает декоратор PublishingStorage через hub.
func (s *MemoryStorage) NotifyTyping(chatID string, user string) error {
	return nil
}
