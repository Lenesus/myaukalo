package sqlite

// NotifyTyping — no-op: typing эфемерный, в БД не хранится.
// Рассылку делает декоратор PublishingStorage через hub.
func (s *Storage) NotifyTyping(chatID string, user string) error {
	return nil
}
