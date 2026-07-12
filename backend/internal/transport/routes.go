package httptransport

import (
	"encoding/json"
	nethttp "net/http"
	"strings"

	"myaukalo/backend/internal/storage/memory"
)

type createChatRequest struct {
	Title string `json:"title"`
}

type createMessageRequest struct {
	Author string `json:"author"`
	Text   string `json:"text"`
}

func RegisterRoutes(mux *nethttp.ServeMux, store *memory.MemoryStorage) {
	mux.HandleFunc("/health", handleHealth)

	mux.HandleFunc("/api/v1/chats", func(w nethttp.ResponseWriter, r *nethttp.Request) {
		handleChats(w, r, store)
	})

	mux.HandleFunc("/api/v1/chats/", func(w nethttp.ResponseWriter, r *nethttp.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/v1/chats/")

		if strings.HasSuffix(path, "/read") {
			chatID := strings.TrimSuffix(path, "/read")
			handleChatRead(w, r, store, chatID)
			return
		}

		if strings.HasSuffix(path, "/messages") {
			handleChatMessages(w, r, store)
			return
		}

		writeJSON(w, nethttp.StatusNotFound, map[string]string{
			"error": "not found",
		})
	})

	mux.HandleFunc("/api/v1/messages", func(w nethttp.ResponseWriter, r *nethttp.Request) {
		handleMessages(w, r, store)
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

func handleChats(w nethttp.ResponseWriter, r *nethttp.Request, store *memory.MemoryStorage) {
	switch r.Method {
	case nethttp.MethodGet:
		chats := store.GetChats()

		writeJSON(w, nethttp.StatusOK, map[string]any{
			"items": chats,
		})

	case nethttp.MethodPost:
		var request createChatRequest

		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			writeJSON(w, nethttp.StatusBadRequest, map[string]string{
				"error": "invalid json",
			})
			return
		}

		title := strings.TrimSpace(request.Title)
		if title == "" {
			writeJSON(w, nethttp.StatusBadRequest, map[string]string{
				"error": "title is required",
			})
			return
		}

		chat := store.AddChat(title)

		writeJSON(w, nethttp.StatusCreated, chat)

	default:
		writeJSON(w, nethttp.StatusMethodNotAllowed, map[string]string{
			"error": "method not allowed",
		})
	}
}

func handleChatMessages(w nethttp.ResponseWriter, r *nethttp.Request, store *memory.MemoryStorage) {
	const prefix = "/api/v1/chats/"
	const suffix = "/messages"

	if !strings.HasPrefix(r.URL.Path, prefix) || !strings.HasSuffix(r.URL.Path, suffix) {
		writeJSON(w, nethttp.StatusNotFound, map[string]string{
			"error": "not found",
		})
		return
	}

	chatID := strings.TrimPrefix(r.URL.Path, prefix)
	chatID = strings.TrimSuffix(chatID, suffix)

	if chatID == "" {
		writeJSON(w, nethttp.StatusBadRequest, map[string]string{
			"error": "chat id is required",
		})
		return
	}

	_, found := store.GetChatByID(chatID)
	if !found {
		writeJSON(w, nethttp.StatusNotFound, map[string]string{
			"error": "chat not found",
		})
		return
	}

	switch r.Method {
	case nethttp.MethodGet:
		messages := store.GetMessagesByChatID(chatID)

		writeJSON(w, nethttp.StatusOK, map[string]any{
			"chat_id": chatID,
			"items":   messages,
		})

	case nethttp.MethodPost:
		var request createMessageRequest

		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			writeJSON(w, nethttp.StatusBadRequest, map[string]string{
				"error": "invalid json",
			})
			return
		}

		author := strings.TrimSpace(request.Author)
		text := strings.TrimSpace(request.Text)

		if author == "" || text == "" {
			writeJSON(w, nethttp.StatusBadRequest, map[string]string{
				"error": "author and text are required",
			})
			return
		}

		message, err := store.AddMessage(chatID, author, text)
		if err != nil {
			writeJSON(w, nethttp.StatusNotFound, map[string]string{
				"error": "chat not found",
			})
			return
		}

		writeJSON(w, nethttp.StatusCreated, message)

	default:
		writeJSON(w, nethttp.StatusMethodNotAllowed, map[string]string{
			"error": "method not allowed",
		})
	}
}

func handleChatRead(w nethttp.ResponseWriter, r *nethttp.Request, store *memory.MemoryStorage, chatID string) {
	if r.Method != nethttp.MethodPost {
		writeJSON(w, nethttp.StatusMethodNotAllowed, map[string]string{
			"error": "method not allowed",
		})
		return
	}

	if chatID == "" {
		writeJSON(w, nethttp.StatusBadRequest, map[string]string{
			"error": "chat id is required",
		})
		return
	}

	chat, found := store.MarkChatAsRead(chatID)
	if !found {
		writeJSON(w, nethttp.StatusNotFound, map[string]string{
			"error": "chat not found",
		})
		return
	}

	writeJSON(w, nethttp.StatusOK, chat)
}

func handleMessages(w nethttp.ResponseWriter, r *nethttp.Request, store *memory.MemoryStorage) {
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
