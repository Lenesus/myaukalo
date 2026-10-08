package httptransport

import (
	"encoding/json"
	"errors"
	nethttp "net/http"
	"strings"
	"time"

	"myaukalo/backend/internal/storage"
)

type createChatRequest struct {
	Title string `json:"title"`
}

type createMessageRequest struct {
	Author string  `json:"author"`
	Text   string  `json:"text"`
	TTL    *string `json:"ttl"`
}

// RegisterRoutes регистрирует HTTP-маршруты приложения
// в переданном маршрутизаторе mux.
func RegisterRoutes(
	mux *nethttp.ServeMux,
	store storage.Storage,
) {
	mux.HandleFunc(
		"/api/v1/chats",
		func(w nethttp.ResponseWriter, r *nethttp.Request) {
			handleChats(w, r, store)
		},
	)

	mux.HandleFunc(
		"/api/v1/chats/",
		func(w nethttp.ResponseWriter, r *nethttp.Request) {
			handleChatRoute(w, r, store)
		},
	)

	mux.HandleFunc(
		"/api/v1/messages",
		func(w nethttp.ResponseWriter, r *nethttp.Request) {
			handleMessages(w, r, store)
		},
	)
}

func handleChats(
	w nethttp.ResponseWriter,
	r *nethttp.Request,
	store storage.Storage,
) {
	switch r.Method {
	case nethttp.MethodGet:
		handleGetChats(w, store)

	case nethttp.MethodPost:
		handleCreateChat(w, r, store)

	default:
		writeJSON(
			w,
			nethttp.StatusMethodNotAllowed,
			map[string]string{
				"error": "method not allowed",
			},
		)
	}
}

func handleGetChats(
	w nethttp.ResponseWriter,
	store storage.Storage,
) {
	chats, err := store.GetChats()
	if err != nil {
		writeStorageError(w, err)
		return
	}

	writeJSON(w, nethttp.StatusOK, chats)
}

func handleCreateChat(
	w nethttp.ResponseWriter,
	r *nethttp.Request,
	store storage.Storage,
) {
	var request createChatRequest

	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		writeJSON(
			w,
			nethttp.StatusBadRequest,
			map[string]string{
				"error": "invalid request body",
			},
		)
		return
	}

	request.Title = strings.TrimSpace(request.Title)

	if request.Title == "" {
		writeJSON(
			w,
			nethttp.StatusBadRequest,
			map[string]string{
				"error": "title is required",
			},
		)
		return
	}

	chat, err := store.AddChat(request.Title)
	if err != nil {
		writeStorageError(w, err)
		return
	}

	writeJSON(w, nethttp.StatusCreated, chat)
}

func handleChatRoute(
	w nethttp.ResponseWriter,
	r *nethttp.Request,
	store storage.Storage,
) {
	path := strings.TrimPrefix(
		r.URL.Path,
		"/api/v1/chats/",
	)

	path = strings.Trim(path, "/")

	if path == "" {
		writeJSON(
			w,
			nethttp.StatusNotFound,
			map[string]string{
				"error": "route not found",
			},
		)
		return
	}

	parts := strings.Split(path, "/")

	if len(parts) == 1 {
		handleChatByID(
			w,
			r,
			store,
			parts[0],
		)
		return
	}

	if len(parts) == 2 && parts[1] == "messages" {
		handleChatMessages(
			w,
			r,
			store,
			parts[0],
		)
		return
	}

	if len(parts) == 2 && parts[1] == "read" {
		handleChatRead(
			w,
			r,
			store,
			parts[0],
		)
		return
	}

	writeJSON(
		w,
		nethttp.StatusNotFound,
		map[string]string{
			"error": "route not found",
		},
	)
}

func handleChatByID(
	w nethttp.ResponseWriter,
	r *nethttp.Request,
	store storage.Storage,
	chatID string,
) {
	if r.Method != nethttp.MethodGet {
		writeJSON(
			w,
			nethttp.StatusMethodNotAllowed,
			map[string]string{
				"error": "method not allowed",
			},
		)
		return
	}

	chat, err := store.GetChatByID(chatID)
	if err != nil {
		writeStorageError(w, err)
		return
	}

	writeJSON(w, nethttp.StatusOK, chat)
}

func handleChatMessages(
	w nethttp.ResponseWriter,
	r *nethttp.Request,
	store storage.Storage,
	chatID string,
) {
	_, err := store.GetChatByID(chatID)
	if err != nil {
		writeStorageError(w, err)
		return
	}

	switch r.Method {
	case nethttp.MethodGet:
		handleGetChatMessages(
			w,
			store,
			chatID,
		)

	case nethttp.MethodPost:
		handleCreateMessage(
			w,
			r,
			store,
			chatID,
		)

	default:
		writeJSON(
			w,
			nethttp.StatusMethodNotAllowed,
			map[string]string{
				"error": "method not allowed",
			},
		)
	}
}

func handleGetChatMessages(
	w nethttp.ResponseWriter,
	store storage.Storage,
	chatID string,
) {
	messages, err := store.GetMessagesByChatID(chatID)
	if err != nil {
		writeStorageError(w, err)
		return
	}

	writeJSON(w, nethttp.StatusOK, messages)
}

func handleCreateMessage(
	w nethttp.ResponseWriter,
	r *nethttp.Request,
	store storage.Storage,
	chatID string,
) {
	var request createMessageRequest

	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		writeJSON(
			w,
			nethttp.StatusBadRequest,
			map[string]string{
				"error": "invalid request body",
			},
		)
		return
	}

	request.Author = strings.TrimSpace(request.Author)
	request.Text = strings.TrimSpace(request.Text)

	if request.Author == "" {
		writeJSON(
			w,
			nethttp.StatusBadRequest,
			map[string]string{
				"error": "author is required",
			},
		)
		return
	}

	if request.Text == "" {
		writeJSON(
			w,
			nethttp.StatusBadRequest,
			map[string]string{
				"error": "text is required",
			},
		)
		return
	}

	var ttl time.Duration
	if request.TTL != nil && *request.TTL != "" {
		parsed, err := time.ParseDuration(*request.TTL)
		if err != nil {
			writeJSON(
				w,
				nethttp.StatusBadRequest,
				map[string]string{
					"error": "invalid ttl",
				},
			)
			return
		}

		if parsed <= 0 {
			writeJSON(
				w,
				nethttp.StatusBadRequest,
				map[string]string{
					"error": "ttl must be positive",
				},
			)
			return
		}

		ttl = parsed
	}

	message, err := store.AddMessage(
		chatID,
		request.Author,
		request.Text,
		ttl,
	)
	if err != nil {
		writeStorageError(w, err)
		return
	}

	writeJSON(
		w,
		nethttp.StatusCreated,
		message,
	)
}

func handleChatRead(
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

	chat, err := store.MarkChatAsRead(chatID)
	if err != nil {
		writeStorageError(w, err)
		return
	}

	writeJSON(w, nethttp.StatusOK, chat)
}

func handleMessages(
	w nethttp.ResponseWriter,
	r *nethttp.Request,
	store storage.Storage,
) {
	if r.Method != nethttp.MethodGet {
		writeJSON(
			w,
			nethttp.StatusMethodNotAllowed,
			map[string]string{
				"error": "method not allowed",
			},
		)
		return
	}

	messages, err := store.GetMessages()
	if err != nil {
		writeStorageError(w, err)
		return
	}

	writeJSON(w, nethttp.StatusOK, messages)
}

func writeStorageError(
	w nethttp.ResponseWriter,
	err error,
) {
	if errors.Is(err, storage.ErrChatNotFound) {
		writeJSON(
			w,
			nethttp.StatusNotFound,
			map[string]string{
				"error": "chat not found",
			},
		)
		return
	}

	writeJSON(
		w,
		nethttp.StatusInternalServerError,
		map[string]string{
			"error": "internal server error",
		},
	)
}

func writeJSON(
	w nethttp.ResponseWriter,
	statusCode int,
	data any,
) {
	w.Header().Set(
		"Content-Type",
		"application/json",
	)

	w.WriteHeader(statusCode)

	_ = json.NewEncoder(w).Encode(data)
}
