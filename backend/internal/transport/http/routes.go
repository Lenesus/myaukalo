package httptransport

import (
	"encoding/json"
	nethttp "net/http"
)

// RegisterRoutes подключает все HTTP-маршруты приложения.
func RegisterRoutes(mux *nethttp.ServeMux) {
	mux.HandleFunc("/health", handleHealth)
	mux.HandleFunc("/api/v1/chats", handleChats)
	mux.HandleFunc("/api/v1/messages", handleMessages)
}

// handleHealth проверяет, что сервер жив.
func handleHealth(w nethttp.ResponseWriter, r *nethttp.Request) {
	if r.Method != nethttp.MethodGet {
		writeJSON(w, nethttp.StatusMethodNotAllowed, map[string]string{
			"error": "method not allowed",
		})
		return
	}

	writeJSON(w, nethttp.StatusOK, map[string]string{
		"status":  "ok",
		"service": "myaukalo-backend",
	})
}

// handleChats — пока заглушка для будущей логики чатов.
func handleChats(w nethttp.ResponseWriter, r *nethttp.Request) {
	switch r.Method {
	case nethttp.MethodGet:
		writeJSON(w, nethttp.StatusOK, map[string]any{
			"items":   []any{},
			"message": "chats list is empty",
		})

	case nethttp.MethodPost:
		writeJSON(w, nethttp.StatusNotImplemented, map[string]string{
			"message": "chat creation is not implemented yet",
		})

	default:
		writeJSON(w, nethttp.StatusMethodNotAllowed, map[string]string{
			"error": "method not allowed",
		})
	}
}

// handleMessages — пока заглушка для будущей логики сообщений.
func handleMessages(w nethttp.ResponseWriter, r *nethttp.Request) {
	switch r.Method {
	case nethttp.MethodGet:
		writeJSON(w, nethttp.StatusOK, map[string]any{
			"items":   []any{},
			"message": "messages list is empty",
		})

	case nethttp.MethodPost:
		writeJSON(w, nethttp.StatusNotImplemented, map[string]string{
			"message": "message sending is not implemented yet",
		})

	default:
		writeJSON(w, nethttp.StatusMethodNotAllowed, map[string]string{
			"error": "method not allowed",
		})
	}
}

// writeJSON отправляет JSON-ответ клиенту.
func writeJSON(w nethttp.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(data)
}
