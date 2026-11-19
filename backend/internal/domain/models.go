package domain

type Chat struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	LastMessage string `json:"last_message"`
	UnreadCount int    `json:"unread_count"`
}

// Reaction — агрегат по эмодзи: кто поставил. Клиент сам считает count=len(Users)
// и "моя ли" через Users.includes(identity.name). Храним построчно (user,emoji),
// агрегируем при чтении.
type Reaction struct {
	Emoji string   `json:"emoji"`
	Users []string `json:"users"`
}

type Message struct {
	ID        string     `json:"id"`
	ChatID    string     `json:"chat_id"`
	Author    string     `json:"author"`
	Text      string     `json:"text"`
	CreatedAt string     `json:"created_at"`
	ExpiresAt *string    `json:"expires_at,omitempty"`
	Reactions []Reaction `json:"reactions,omitempty"`
}
