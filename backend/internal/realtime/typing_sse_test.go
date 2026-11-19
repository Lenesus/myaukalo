package realtime_test

import (
	"bufio"
	"context"
	nethttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"myaukalo/backend/internal/realtime"
	"myaukalo/backend/internal/storage/memory"
)

func TestSSE_TypingEvent(t *testing.T) {
	hub := realtime.NewHub()
	defer hub.Close()

	store := memory.NewMemoryStorage()
	srv := httptest.NewServer(realtime.HandleEvents(hub, store))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, err := nethttp.NewRequestWithContext(ctx, nethttp.MethodGet, srv.URL+"?chat_id=chat-1", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	resp, err := nethttp.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != nethttp.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	reader := bufio.NewReader(resp.Body)

	// Ждём строку retry: она пишется ПОСЛЕ подписки, значит подписка активна.
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read retry: %v", err)
		}
		if strings.HasPrefix(line, "retry:") {
			break
		}
	}

	hub.PublishTyping("chat-1", "alice")

	sawEvent := false
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read typing event: %v", err)
		}
		line = strings.TrimSpace(line)

		if line == "event: typing" {
			sawEvent = true
			continue
		}
		if sawEvent && strings.HasPrefix(line, "data:") {
			if !strings.Contains(line, "alice") {
				t.Fatalf("unexpected typing payload: %s", line)
			}
			return
		}
	}
}
