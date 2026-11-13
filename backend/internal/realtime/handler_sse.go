package realtime

import (
	"encoding/json"
	"fmt"
	nethttp "net/http"

	"myaukalo/backend/internal/storage"
)

// HandleEvents обслуживает GET /api/v1/events?chat_id=<id> как SSE-поток.
//
// Важно: все проверки валидности выполняются ДО подписки на hub и ДО входа
// в бесконечный select-цикл. Иначе невалидный запрос может навсегда зависнуть
// в стриме, как это произошло в TestSSE_RejectsUnknownChat.
func HandleEvents(hub *Hub, store storage.Storage) nethttp.HandlerFunc {
	return func(w nethttp.ResponseWriter, r *nethttp.Request) {
		if r.Method != nethttp.MethodGet {
			writeSSEError(w, nethttp.StatusMethodNotAllowed, "method not allowed")
			return
		}

		chatID := r.URL.Query().Get("chat_id")
		if chatID == "" {
			writeSSEError(w, nethttp.StatusBadRequest, "chat_id is required")
			return
		}

		if _, err := store.GetChatByID(chatID); err != nil {
			writeSSEError(w, nethttp.StatusNotFound, "chat not found")
			return
		}

		flusher, ok := w.(nethttp.Flusher)
		if !ok {
			writeSSEError(w, nethttp.StatusInternalServerError, "streaming unsupported")
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")

		ch := hub.Subscribe(chatID)
		defer hub.Unsubscribe(chatID, ch)

		if _, err := fmt.Fprint(w, "retry: 3000\n\n"); err != nil {
			return
		}
		flusher.Flush()

		ctx := r.Context()

		for {
			select {
			case <-ctx.Done():
				return
			case <-hub.Done():
				return
			case msg := <-ch:
				data, err := json.Marshal(msg)
				if err != nil {
					continue
				}

				if _, err := fmt.Fprintf(w, "event: message\ndata: %s\n\n", data); err != nil {
					return
				}
				flusher.Flush()
			}
		}
	}
}

// writeSSEError отправляет обычное JSON-ошибочное risposta до начала SSE-стрима.
func writeSSEError(w nethttp.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
