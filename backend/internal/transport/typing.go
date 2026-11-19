package httptransport

import (
	"encoding/json"
	nethttp "net/http"
	"strings"

	"myaukalo/backend/internal/storage"
)

type typingRequest struct {
	User string `json:"user"`
}

// handleChatTyping обслуживает POST /api/v1/chats/{chatID}/typing.
// Эфемерный сигнал «печатает…»: не хранится, только рассылается через hub
// (декоратор PublishingStorage.NotifyTyping). Ответ 204 без тела.
func handleChatTyping(
	w nethttp.ResponseWriter,
	r *nethttp.Request,
	store storage.Storage,
	chatID string,
) {
	if r.Method != nethttp.MethodPost {
		writeJSON(
			w,
			nethttp.StatusMethodNotAllowed,
			map[string]string{
				"error": "method not allowed",
			},
		)
		return
	}

	var request typingRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(
			w,
			nethttp.StatusBadRequest,
			map[string]string{
				"error": "invalid request body",
			},
		)
		return
	}

	request.User = strings.TrimSpace(request.User)

	if request.User == "" {
		writeJSON(
			w,
			nethttp.StatusBadRequest,
			map[string]string{
				"error": "user is required",
			},
		)
		return
	}

	// Чат должен существовать, иначе 404 (не шлём typing в пустоту).
	if _, err := store.GetChatByID(chatID); err != nil {
		writeStorageError(w, err)
		return
	}

	if err := store.NotifyTyping(chatID, request.User); err != nil {
		writeStorageError(w, err)
		return
	}

	w.WriteHeader(nethttp.StatusNoContent)
}
