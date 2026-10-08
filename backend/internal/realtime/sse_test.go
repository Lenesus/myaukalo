package realtime

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	nethttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"myaukalo/backend/internal/domain"
	"myaukalo/backend/internal/storage"
)

var errSSETimeout = errors.New("sse: read timeout")

func TestHub_PublishDeliversToSubscribers(t *testing.T) {
	hub := NewHub()
	defer hub.Close()

	sub := hub.Subscribe("chat-1")
	hub.Publish(domain.Message{ChatID: "chat-1", Text: "мяу"})

	select {
	case message := <-sub:
		if message.Text != "мяу" {
			t.Fatalf("expected text %q, got %q", "мяу", message.Text)
		}
	case <-time.After(time.Second):
		t.Fatal("subscriber did not receive published message")
	}
}

func TestHub_UnsubscribeStopsDelivery(t *testing.T) {
	hub := NewHub()
	defer hub.Close()

	sub := hub.Subscribe("chat-1")
	hub.Unsubscribe("chat-1", sub)
	hub.Publish(domain.Message{ChatID: "chat-1", Text: "в пустоту"})

	select {
	case message := <-sub:
		t.Fatalf("received message after unsubscribe: %+v", message)
	default:
	}
}

func TestHub_ChatIsolation(t *testing.T) {
	hub := NewHub()
	defer hub.Close()

	subA := hub.Subscribe("chat-A")
	subB := hub.Subscribe("chat-B")

	hub.Publish(domain.Message{ChatID: "chat-B", Text: "только для B"})

	select {
	case message := <-subA:
		t.Fatalf("chat A leaked message from B: %+v", message)
	default:
	}

	select {
	case message := <-subB:
		if message.Text != "только для B" {
			t.Fatalf("unexpected message in B: %+v", message)
		}
	case <-time.After(time.Second):
		t.Fatal("chat B did not receive its message")
	}
}

func TestHub_PublishNonBlockingOnFullBuffer(t *testing.T) {
	hub := NewHub()
	defer hub.Close()

	sub := hub.Subscribe("chat-1")
	_ = sub

	for i := 0; i < 16; i++ {
		hub.Publish(domain.Message{ChatID: "chat-1", Text: "fill"})
	}

	done := make(chan struct{}, 1)
	go func() {
		hub.Publish(domain.Message{ChatID: "chat-1", Text: "overflow"})
		done <- struct{}{}
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Publish blocked on full subscriber buffer")
	}
}

func TestHub_CloseSignalsDone(t *testing.T) {
	hub := NewHub()
	hub.Close()

	select {
	case <-hub.Done():
	default:
		t.Fatal("Done channel not closed after Close()")
	}

	hub.Close()
}

func TestPublishingStorage_AddMessagePublishes(t *testing.T) {
	hub := NewHub()
	defer hub.Close()

	fake := newFakeStorage("chat-1")
	publishing := NewPublishingStorage(fake, hub)

	sub := hub.Subscribe("chat-1")
	message, err := publishing.AddMessage("chat-1", "Барсик", "мяу live", 0)
	if err != nil {
		t.Fatalf("AddMessage: %v", err)
	}
	if fake.addCalls != 1 {
		t.Fatalf("expected inner.AddMessage called once, got %d", fake.addCalls)
	}
	if message.ExpiresAt != nil {
		t.Fatalf("ttl=0 must produce permanent message, got expires_at=%v", *message.ExpiresAt)
	}

	select {
	case published := <-sub:
		if published.ID != message.ID || published.Text != "мяу live" {
			t.Fatalf("published message mismatch:\nwant %+v\n got %+v", message, published)
		}
	case <-time.After(time.Second):
		t.Fatal("successful AddMessage did not publish to hub")
	}
}

func TestPublishingStorage_AddMessagePublishesExpiring(t *testing.T) {
	hub := NewHub()
	defer hub.Close()

	fake := newFakeStorage("chat-1")
	publishing := NewPublishingStorage(fake, hub)

	sub := hub.Subscribe("chat-1")
	message, err := publishing.AddMessage("chat-1", "Барсик", "сгорит скоро", time.Minute)
	if err != nil {
		t.Fatalf("AddMessage: %v", err)
	}
	if message.ExpiresAt == nil {
		t.Fatal("ttl > 0 must produce expires_at")
	}

	select {
	case published := <-sub:
		if published.ExpiresAt == nil || *published.ExpiresAt != *message.ExpiresAt {
			t.Fatalf("published expiring message lost expires_at: %+v", published)
		}
	case <-time.After(time.Second):
		t.Fatal("expiring AddMessage did not publish to hub")
	}
}

func TestPublishingStorage_ErrorDoesNotPublish(t *testing.T) {
	hub := NewHub()
	defer hub.Close()

	fake := newFakeStorage("chat-1")
	fake.addErr = errors.New("disk on fire")
	publishing := NewPublishingStorage(fake, hub)

	sub := hub.Subscribe("chat-1")
	if _, err := publishing.AddMessage("chat-1", "Барсик", "мяу", 0); err == nil {
		t.Fatal("expected error from inner storage")
	}

	select {
	case message := <-sub:
		t.Fatalf("must not publish when inner AddMessage failed: %+v", message)
	default:
	}
}

func TestSSE_HappyPath(t *testing.T) {
	hub := NewHub()
	defer hub.Close()

	fake := newFakeStorage("chat-1")
	publishing := NewPublishingStorage(fake, hub)

	srv := httptest.NewServer(HandleEvents(hub, publishing))
	defer srv.Close()

	resp, reader, cancel := openSSE(t, srv, "chat-1")
	defer cancel()
	defer resp.Body.Close()

	frame, err := readFrame(reader, 2*time.Second)
	if err != nil {
		t.Fatalf("reading retry frame: %v", err)
	}
	if frame.Retry != "3000" {
		t.Fatalf("expected retry:3000 first, got frame %+v", frame)
	}
	if count := hub.SubscriberCount("chat-1"); count != 1 {
		t.Fatalf("expected 1 subscriber after stream open, got %d", count)
	}

	if _, err := publishing.AddMessage("chat-1", "Барсик", "мяу в реальном времени", 0); err != nil {
		t.Fatalf("AddMessage: %v", err)
	}

	message := waitForMessage(t, reader, 3*time.Second)
	if message.Author != "Барсик" || message.Text != "мяу в реальном времени" {
		t.Fatalf("unexpected streamed message: %+v", message)
	}
}

func TestSSE_ChatIsolationOverHTTP(t *testing.T) {
	hub := NewHub()
	defer hub.Close()

	fake := newFakeStorage("chat-A", "chat-B")
	publishing := NewPublishingStorage(fake, hub)

	srv := httptest.NewServer(HandleEvents(hub, publishing))
	defer srv.Close()

	resp, reader, cancel := openSSE(t, srv, "chat-A")
	defer cancel()
	defer resp.Body.Close()

	if _, err := readFrame(reader, 2*time.Second); err != nil {
		t.Fatalf("reading retry frame: %v", err)
	}

	if _, err := publishing.AddMessage("chat-B", "Мурка", "секрет Б", 0); err != nil {
		t.Fatalf("AddMessage B: %v", err)
	}

	if _, err := readFrame(reader, 300*time.Millisecond); !errors.Is(err, errSSETimeout) {
		t.Fatalf("chat A must not receive chat B events, got err=%v", err)
	}
}

func TestSSE_ClientDisconnectCleansUp(t *testing.T) {
	hub := NewHub()
	defer hub.Close()

	fake := newFakeStorage("chat-1")
	publishing := NewPublishingStorage(fake, hub)

	srv := httptest.NewServer(HandleEvents(hub, publishing))
	defer srv.Close()

	resp, reader, cancel := openSSE(t, srv, "chat-1")
	defer resp.Body.Close()

	if _, err := readFrame(reader, 2*time.Second); err != nil {
		t.Fatalf("reading retry frame: %v", err)
	}
	if count := hub.SubscriberCount("chat-1"); count != 1 {
		t.Fatalf("expected 1 subscriber before disconnect, got %d", count)
	}

	cancel()

	waitForSubscriberCount(t, hub, "chat-1", 0, 2*time.Second)

	if _, err := publishing.AddMessage("chat-1", "Рыжик", "в эфир без слушателей", 0); err != nil {
		t.Fatalf("AddMessage after disconnect: %v", err)
	}
}

func TestSSE_RejectsNonGet(t *testing.T) {
	hub := NewHub()
	defer hub.Close()

	handler := HandleEvents(hub, newFakeStorage("chat-1"))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(nethttp.MethodPost, "/api/v1/events?chat_id=chat-1", nil)
	handler(recorder, request)

	assertSSError(t, recorder, nethttp.StatusMethodNotAllowed, "method not allowed")
}

func TestSSE_RejectsMissingChatID(t *testing.T) {
	hub := NewHub()
	defer hub.Close()

	handler := HandleEvents(hub, newFakeStorage("chat-1"))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(nethttp.MethodGet, "/api/v1/events", nil)
	handler(recorder, request)

	assertSSError(t, recorder, nethttp.StatusBadRequest, "chat_id is required")
}

func TestSSE_RejectsUnknownChat(t *testing.T) {
	hub := NewHub()
	defer hub.Close()

	handler := HandleEvents(hub, newFakeStorage("chat-1"))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(nethttp.MethodGet, "/api/v1/events?chat_id=ghost", nil)
	handler(recorder, request)

	assertSSError(t, recorder, nethttp.StatusNotFound, "chat not found")
}

type sseFrame struct {
	Event string
	Data  string
	Retry string
}

func readFrame(reader *bufio.Reader, timeout time.Duration) (sseFrame, error) {
	type result struct {
		frame sseFrame
		err   error
	}

	ch := make(chan result, 1)

	go func() {
		frame, err := parseFrame(reader)
		ch <- result{frame: frame, err: err}
	}()

	select {
	case got := <-ch:
		return got.frame, got.err
	case <-time.After(timeout):
		return sseFrame{}, errSSETimeout
	}
}

func parseFrame(reader *bufio.Reader) (sseFrame, error) {
	var frame sseFrame
	var dataLines []string

	for {
		line, err := reader.ReadString('\n')
		if err != nil && line == "" {
			return sseFrame{}, err
		}

		line = strings.TrimRight(line, "\r\n")

		if line == "" {
			frame.Data = strings.Join(dataLines, "\n")
			return frame, nil
		}

		if strings.HasPrefix(line, ":") {
			continue
		}

		field, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}

		if len(value) > 0 && value[0] == ' ' {
			value = value[1:]
		}

		switch field {
		case "event":
			frame.Event = value
		case "data":
			dataLines = append(dataLines, value)
		case "retry":
			frame.Retry = value
		}
	}
}

func waitForMessage(t *testing.T, reader *bufio.Reader, timeout time.Duration) domain.Message {
	t.Helper()

	deadline := time.Now().Add(timeout)

	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			t.Fatal("timed out waiting for message event")
		}

		frame, err := readFrame(reader, remaining)
		if err != nil {
			if errors.Is(err, errSSETimeout) {
				t.Fatal("timed out waiting for message event")
			}
			t.Fatalf("read frame: %v", err)
		}

		if frame.Event != "message" {
			continue
		}

		var message domain.Message
		if err := json.Unmarshal([]byte(frame.Data), &message); err != nil {
			t.Fatalf("decode message data %q: %v", frame.Data, err)
		}

		return message
	}
}

func openSSE(t *testing.T, srv *httptest.Server, chatID string) (*nethttp.Response, *bufio.Reader, context.CancelFunc) {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	url := srv.URL + "/api/v1/events?chat_id=" + chatID

	request, err := nethttp.NewRequestWithContext(ctx, nethttp.MethodGet, url, nil)
	if err != nil {
		cancel()
		t.Fatalf("build request: %v", err)
	}

	response, err := srv.Client().Do(request)
	if err != nil {
		cancel()
		t.Fatalf("do request: %v", err)
	}

	if response.StatusCode != nethttp.StatusOK {
		_ = response.Body.Close()
		cancel()
		t.Fatalf("expected 200, got %d", response.StatusCode)
	}

	contentType := response.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "text/event-stream") {
		_ = response.Body.Close()
		cancel()
		t.Fatalf("expected text/event-stream, got %q", contentType)
	}

	return response, bufio.NewReader(response.Body), cancel
}

func waitForSubscriberCount(t *testing.T, hub *Hub, chatID string, wanted int, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)

	for {
		if got := hub.SubscriberCount(chatID); got == wanted {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("subscriber count for %s never reached %d (last=%d)",
				chatID, wanted, hub.SubscriberCount(chatID))
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func assertSSError(t *testing.T, recorder *httptest.ResponseRecorder, wantStatus int, wantMessage string) {
	t.Helper()

	if recorder.Code != wantStatus {
		t.Fatalf("expected status %d, got %d (body=%s)", wantStatus, recorder.Code, recorder.Body.String())
	}

	if !strings.Contains(recorder.Body.String(), wantMessage) {
		t.Fatalf("expected body to contain %q, got %s", wantMessage, recorder.Body.String())
	}
}

type fakeStorage struct {
	known    map[string]bool
	addErr   error
	addCalls int
}

var _ storage.Storage = (*fakeStorage)(nil)

func newFakeStorage(ids ...string) *fakeStorage {
	known := make(map[string]bool, len(ids))
	for _, id := range ids {
		known[id] = true
	}
	return &fakeStorage{known: known}
}

func (f *fakeStorage) GetChats() ([]domain.Chat, error) {
	return nil, nil
}

func (f *fakeStorage) GetChatByID(chatID string) (domain.Chat, error) {
	if f.known[chatID] {
		return domain.Chat{ID: chatID, Title: "fake"}, nil
	}
	return domain.Chat{}, storage.ErrChatNotFound
}

func (f *fakeStorage) AddChat(_ string) (domain.Chat, error) {
	return domain.Chat{}, nil
}

func (f *fakeStorage) MarkChatAsRead(_ string) (domain.Chat, error) {
	return domain.Chat{}, nil
}

func (f *fakeStorage) GetMessages() ([]domain.Message, error) {
	return nil, nil
}

func (f *fakeStorage) GetMessagesByChatID(_ string) ([]domain.Message, error) {
	return nil, nil
}

func (f *fakeStorage) AddMessage(chatID string, author string, text string, ttl time.Duration) (domain.Message, error) {
	if f.addErr != nil {
		return domain.Message{}, f.addErr
	}
	if !f.known[chatID] {
		return domain.Message{}, storage.ErrChatNotFound
	}

	f.addCalls++

	message := domain.Message{
		ID:        "fake-msg",
		ChatID:    chatID,
		Author:    author,
		Text:      text,
		CreatedAt: "2026-10-01T00:00:00Z",
	}

	if ttl > 0 {
		expiresAt := time.Now().UTC().Add(ttl).Format(time.RFC3339)
		message.ExpiresAt = &expiresAt
	}

	return message, nil
}
