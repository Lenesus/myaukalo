package httptransport

import (
	"encoding/json"
	nethttp "net/http"

	"myaukalo/backend/internal/storage"
)

func RegisterRoutes(mux *nethttp.ServeMux, store *storage.MemoryStorage) {
	mux.HandleFunc("/health", handleHealth)

	mux.HandleFunc("/api/v1/chats", func(w nethttp.ResponseWriter, r *nethttp.Request) {
		handleChats(w, r, store)
	})

	mux.HandleFunc("/api/v1/chats/", func(w nethttp.ResponseWriter, r *nethttp.Request) {
			handleChatMessages(w, r, store)
		})


	mux.HandleFunc("/api/v1/messages", func(w nethttp.ResponseWriter, r *nethttp.Request) {
		handleMessages(w, r, store)
	})
}

func handleChatMessages(w nethttp.ResponseWriter, r *nethttp.Request, store *storage.MemoryStorage) {
	if r.Method != nethttp.MethodGet {
		writeJSON(w, nethttp.StatusMethodNotAllowed, map[string]string{
			"error": "method not allowed",
		})
		return
	}

	chatID := r.URL.Path[len("/api/v1/chats/"):]
	chatID = chaID[:len(chatID)-len("/messages")]

	messages := store.GetMessagesByChatID(chatID)

	writeJSON(w, nethttp.StatusOK, map[string]any{
		"chat__id": chatID,
		"items": messages,
	})

}

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

func handleChats(w nethttp.ResponseWriter, r *nethttp.Request, store *storage.MemoryStorage) {
	switch r.Method {
	case nethttp.MethodGet:
		chats := store.GetChats()

		writeJSON(w, nethttp.StatusOK, map[string]any{
			"items": chats,
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

func handleMessages(w nethttp.ResponseWriter, r *nethttp.Request, store *storage.MemoryStorage) {
	switch r.Method {
	case nethttp.MethodGet:
		messages := store.GetMessages()

		writeJSON(w, nethttp.StatusOK, map[string]any{
			"items": messages,
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

func writeJSON(w nethttp.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(data)
}
