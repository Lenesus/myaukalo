package realtime

import (
	"encoding/json"
	"fmt"
	nethttp "net/http"

	"myaukalo/backend/internal/storage"
)

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

		fmt.Fprintf(w, "retry: 3000\n\n")
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
				fmt.Fprintf(w, "event: message\ndata: %s\n\n", data)
				flusher.Flush()
			}
		}
	}

}

func writeSSEError(w nethttp.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
