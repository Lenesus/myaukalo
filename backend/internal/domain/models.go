package domain

type Chat struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	LastMessage string `json:"last_message"`
	UnreadCount int    `json:"unread_count"`
}

type Message struct {
	ID        string `json:"id"`
	ChatID    string `json:"chat_id"`
	Author    string `json:"author"`
	Text      string `json:"text"`
	CreatedAt string `json:"created_at"`
}
