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

// errSSETimeout — sentinel: чтение фрейма не уложилось в отведённое время.
// Сравниваем через errors.Is, поэтому никогда не оборачиваем его в fmt.Errorf.
var errSSETimeout = errors.New("sse: read timeout")

// ============================================================================
// СЕКЦИЯ 1. Чистый Hub (без HTTP) — ядро pub/sub.
// ============================================================================

func TestHub_PublishDeliversToSubscribers(t *testing.T) {
	hub := NewHub()
	defer hub.Close()

	sub := hub.Subscribe("chat-1")
	hub.Publish(domain.Message{ChatID: "chat-1", Text: "мяу"})

	select {
	case msg := <-sub:
		if msg.Text != "мяу" {
			t.Fatalf("expected text %q, got %q", "мяу", msg.Text)
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

	// select+default: проверяем, что канал пуст, НЕ блокируясь.
	select {
	case msg := <-sub:
		t.Fatalf("received message after unsubscribe: %+v", msg)
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
	case msg := <-subA:
		t.Fatalf("chat A leaked message from B: %+v", msg)
	default:
	}

	select {
	case msg := <-subB:
		if msg.Text != "только для B" {
			t.Fatalf("unexpected message in B: %+v", msg)
		}
	case <-time.After(time.Second):
		t.Fatal("chat B did not receive its message")
	}
}

func TestHub_PublishNonBlockingOnFullBuffer(t *testing.T) {
	hub := NewHub()
	defer hub.Close()

	// Подписчик есть, но читателя нет — забиваем буфер (ёмкость 16) до отказа.
	sub := hub.Subscribe("chat-1")
	_ = sub
	for i := 0; i < 16; i++ {
		hub.Publish(domain.Message{ChatID: "chat-1", Text: "fill"})
	}

	// 17-й Publish при полном буфере обязан НЕ заблокировать издателя
	// (внутри select+default). Доказываем это, запуская в горутине с таймаутом.
	done := make(chan struct{}, 1) // буфер 1 => горутина выйдет даже если тест уже упал
	go func() {
		hub.Publish(domain.Message{ChatID: "chat-1", Text: "overflow"})
		done <- struct{}{}
	}()

	select {
	case <-done:
		// ок, не заблокировался
	case <-time.After(time.Second):
		t.Fatal("Publish blocked on full subscriber buffer")
	}
}

func TestHub_CloseSignalsDone(t *testing.T) {
	hub := NewHub()
	hub.Close()

	// После Close канал Done закрыт => чтение из него всегда «готово».
	select {
	case <-hub.Done():
	default:
		t.Fatal("Done channel not closed after Close()")
	}

	// Идемпотентность: повторный Close не должен паниковать.
	hub.Close()
}

// ============================================================================
// СЕКЦИЯ 2. PublishingStorage (декоратор) — без HTTP, на фейковом хранилище.
// ============================================================================

func TestPublishingStorage_AddMessagePublishes(t *testing.T) {
	hub := NewHub()
	defer hub.Close()

	fs := newFakeStorage("chat-1")
	ps := NewPublishingStorage(fs, hub)

	sub := hub.Subscribe("chat-1")
	msg, err := ps.AddMessage("chat-1", "Барсик", "мяу live")
	if err != nil {
		t.Fatalf("AddMessage: %v", err)
	}
	if fs.addCalls != 1 {
		t.Fatalf("expected inner.AddMessage called once, got %d", fs.addCalls)
	}

	select {
	case got := <-sub:
		if got.ID != msg.ID || got.Text != "мяу live" {
			t.Fatalf("published message mismatch:\nwant %+v\n got %+v", msg, got)
		}
	case <-time.After(time.Second):
		t.Fatal("successful AddMessage did not publish to hub")
	}
}

func TestPublishingStorage_ErrorDoesNotPublish(t *testing.T) {
	hub := NewHub()
	defer hub.Close()

	fs := newFakeStorage("chat-1")
	fs.addErr = errors.New("disk on fire")
	ps := NewPublishingStorage(fs, hub)

	sub := hub.Subscribe("chat-1")
	if _, err := ps.AddMessage("chat-1", "Барсик", "мяу"); err == nil {
		t.Fatal("expected error from inner storage")
	}

	// Ошибка => публикации не было => канал пуст.
	select {
	case msg := <-sub:
		t.Fatalf("must not publish when inner AddMessage failed: %+v", msg)
	default:
	}
}

// ============================================================================
// СЕКЦИЯ 3. HandleEvents (настоящий SSE по HTTP) — httptest.NewServer.
// ============================================================================

func TestSSE_HappyPath(t *testing.T) {
	hub := NewHub()
	defer hub.Close()
	fs := newFakeStorage("chat-1")
	ps := NewPublishingStorage(fs, hub)

	srv := httptest.NewServer(HandleEvents(hub, ps))
	defer srv.Close()

	resp, br, cancel := openSSE(t, srv, "chat-1")
	defer cancel()
	defer resp.Body.Close()

	// Первый фрейм — retry (сервер шлёт его сразу). Его получение = стрим жив,
	// а подписчик уже зарегистрирован (Subscribe вызывается до записи retry).
	frame, err := readFrame(br, 2*time.Second)
	if err != nil {
		t.Fatalf("reading retry frame: %v", err)
	}
	if frame.Retry != "3000" {
		t.Fatalf("expected retry:3000 first, got frame %+v", frame)
	}
	if n := hub.SubscriberCount("chat-1"); n != 1 {
		t.Fatalf("expected 1 subscriber after stream open, got %d", n)
	}

	// Публикуем сообщение «в боковом окне» — как это сделает POST /messages.
	if _, err := ps.AddMessage("chat-1", "Барсик", "мяу в реальном времени"); err != nil {
		t.Fatalf("AddMessage: %v", err)
	}

	msg := waitForMessage(t, br, 3*time.Second)
	if msg.Author != "Барсик" || msg.Text != "мяу в реальном времени" {
		t.Fatalf("unexpected streamed message: %+v", msg)
	}
}

func TestSSE_ChatIsolationOverHTTP(t *testing.T) {
	hub := NewHub()
	defer hub.Close()
	fs := newFakeStorage("chat-A", "chat-B")
	ps := NewPublishingStorage(fs, hub)

	srv := httptest.NewServer(HandleEvents(hub, ps))
	defer srv.Close()

	// Подписываемся ТОЛЬКО на chat-A.
	resp, br, cancel := openSSE(t, srv, "chat-A")
	defer cancel()
	defer resp.Body.Close()
	if _, err := readFrame(br, 2*time.Second); err != nil { // съедаем retry
		t.Fatalf("reading retry frame: %v", err)
	}

	// Пишем в chat-B — подписчик A не должен ничего увидеть.
	if _, err := ps.AddMessage("chat-B", "Мурка", "секрет Б"); err != nil {
		t.Fatalf("AddMessage B: %v", err)
	}

	// Негативная проверка: ожидаем ТАЙМАУТ (ни одного message-фрейма).
	if _, err := readFrame(br, 300*time.Millisecond); !errors.Is(err, errSSETimeout) {
		t.Fatalf("chat A must not receive chat B events, got err=%v", err)
	}
}

func TestSSE_ClientDisconnectCleansUp(t *testing.T) {
	hub := NewHub()
	defer hub.Close()
	fs := newFakeStorage("chat-1")
	ps := NewPublishingStorage(fs, hub)

	srv := httptest.NewServer(HandleEvents(hub, ps))
	defer srv.Close()

	resp, br, cancel := openSSE(t, srv, "chat-1")
	defer resp.Body.Close()

	if _, err := readFrame(br, 2*time.Second); err != nil { // стрим открыт, подписчик есть
		t.Fatalf("reading retry frame: %v", err)
	}
	if n := hub.SubscriberCount("chat-1"); n != 1 {
		t.Fatalf("expected 1 subscriber before disconnect, got %d", n)
	}

	cancel() // эмулируем закрытие вкладкой браузером

	// Cleanup асинхронный (defer в горутине net/http) => ждём через polling.
	waitForSubscriberCount(t, hub, "chat-1", 0, 2*time.Second)

	// Публикация после отключения не должна паниковать и вешать сервер.
	if _, err := ps.AddMessage("chat-1", "Рыжик", "в эфир без слушателей"); err != nil {
		t.Fatalf("AddMessage after disconnect: %v", err)
	}
}

// --- Валидации ДО стрима: тут handler возвращается сразу, поэтому Recorder уместен. ---

func TestSSE_RejectsNonGet(t *testing.T) {
	hub := NewHub()
	defer hub.Close()
	handler := HandleEvents(hub, newFakeStorage("chat-1"))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(nethttp.MethodPost, "/api/v1/events?chat_id=chat-1", nil)
	handler(rec, req)

	assertSSError(t, rec, nethttp.StatusMethodNotAllowed, "method not allowed")
}

func TestSSE_RejectsMissingChatID(t *testing.T) {
	hub := NewHub()
	defer hub.Close()
	handler := HandleEvents(hub, newFakeStorage("chat-1"))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(nethttp.MethodGet, "/api/v1/events", nil)
	handler(rec, req)

	assertSSError(t, rec, nethttp.StatusBadRequest, "chat_id is required")
}

func TestSSE_RejectsUnknownChat(t *testing.T) {
	hub := NewHub()
	defer hub.Close()
	handler := HandleEvents(hub, newFakeStorage("chat-1"))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(nethttp.MethodGet, "/api/v1/events?chat_id=ghost", nil)
	handler(rec, req)

	assertSSError(t, rec, nethttp.StatusNotFound, "chat not found")
}

// ============================================================================
// ХЕЛПЕРЫ. Именно здесь живут anti-flaky приёмы — читай внимательно.
// ============================================================================

// sseFrame — разобранный SSE-фрейм (блок строк до пустой линии).
type sseFrame struct {
	Event string
	Data  string
	Retry string
}

// readFrame читает ОДИН фрейм с таймаутом.
//
// Приём anti-flaky: парсинг идёт в горутине, результат пишется в КАНАЛ С БУФЕРОМ 1.
// Буфер 1 критичен: даже если тест ушёл по таймауту (t.Fatal => Goexit), горутина
// всё равно запишет результат в буфер и завершится, а не зависнет навсегда на
// блокирующем Read. Без буфера получили бы утечку горутин в тесте.
func readFrame(br *bufio.Reader, timeout time.Duration) (sseFrame, error) {
	type result struct {
		frame sseFrame
		err   error
	}
	ch := make(chan result, 1) // buffer 1 => goroutine always exits

	go func() {
		f, err := parseFrame(br)
		ch <- result{f, err}
	}()

	select {
	case r := <-ch:
		return r.frame, r.err
	case <-time.After(timeout):
		return sseFrame{}, errSSETimeout
	}
}

// parseFrame собирает строки до пустой линии в sseFrame по спецификации SSE:
//   - строка ""            => конец фрейма;
//   - строка ":..."        => комментарий, игнорируем;
//   - "field: value"       => поле; один ведущий пробел после ':' убирается.
func parseFrame(br *bufio.Reader) (sseFrame, error) {
	var f sseFrame
	var dataLines []string

	for {
		line, err := br.ReadString('\n')
		if err != nil && line == "" {
			return sseFrame{}, err // EOF/ошибка сети до конца фрейма
		}
		line = strings.TrimRight(line, "\r\n")

		if line == "" { // граница события
			f.Data = strings.Join(dataLines, "\n")
			return f, nil
		}
		if strings.HasPrefix(line, ":") { // комментарий
			continue
		}

		field, value, ok := strings.Cut(line, ":")
		if !ok {
			continue // malformed line без ':' — игнорируем по спеке
		}
		if len(value) > 0 && value[0] == ' ' { // ровно один ведущий пробел
			value = value[1:]
		}

		switch field {
		case "event":
			f.Event = value
		case "data":
			dataLines = append(dataLines, value)
		case "retry":
			f.Retry = value
		}
	}
}

// waitForMessage пропускает служебные фреймы (retry и пр.) и возвращает данные
// первого фрейма с event=="message". Общий таймаут дедлайном, чтобы не зависеть
// от числа промежуточных фреймов.
func waitForMessage(t *testing.T, br *bufio.Reader, timeout time.Duration) domain.Message {
	t.Helper()
	deadline := time.Now().Add(timeout)

	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			t.Fatal("timed out waiting for message event")
		}
		frame, err := readFrame(br, remaining)
		if err != nil {
			if errors.Is(err, errSSETimeout) {
				t.Fatal("timed out waiting for message event")
			}
			t.Fatalf("read frame: %v", err)
		}
		if frame.Event != "message" {
			continue // retry/heartbeat/прочее — не то, что ищем
		}

		var msg domain.Message
		if err := json.Unmarshal([]byte(frame.Data), &msg); err != nil {
			t.Fatalf("decode message data %q: %v", frame.Data, err)
		}
		return msg
	}
}

// openSSE открывает стрим и возвращает response, reader и cancel.
// При любой ошибке до возврата — аккуратно закрываем, чтобы не течь.
func openSSE(t *testing.T, srv *httptest.Server, chatID string) (*nethttp.Response, *bufio.Reader, context.CancelFunc) {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	url := srv.URL + "/api/v1/events?chat_id=" + chatID // chatID контролируем, экранирование не нужно

	req, err := nethttp.NewRequestWithContext(ctx, nethttp.MethodGet, url, nil)
	if err != nil {
		cancel()
		t.Fatalf("build request: %v", err)
	}

	resp, err := srv.Client().Do(req)
	if err != nil {
		cancel()
		t.Fatalf("do request: %v", err)
	}
	if resp.StatusCode != nethttp.StatusOK {
		_ = resp.Body.Close()
		cancel()
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		_ = resp.Body.Close()
		cancel()
		t.Fatalf("expected text/event-stream, got %q", ct)
	}

	return resp, bufio.NewReader(resp.Body), cancel
}

// waitForSubscriberCount — polling-хелпер для асинхронного cleanup.
// После cancel() defer-Unsubscribe в горутине net/http отрабатывает НЕ мгновенно,
// поэтому крутим цикл с маленьким шагом, пока счётчик не станет wanted или не
// истечёт таймаут. Это детерминирует иначе flaky-тест «sleep(100ms) и надейся».
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

func assertSSError(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int, wantMsg string) {
	t.Helper()
	if rec.Code != wantStatus {
		t.Fatalf("expected status %d, got %d (body=%s)", wantStatus, rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), wantMsg) {
		t.Fatalf("expected body to contain %q, got %s", wantMsg, rec.Body.String())
	}
}

// ============================================================================
// FAKE STORAGE — минимальная заглушка storage.Storage.
// Учимся не тащить SQLite в HTTP-тесты: хендлеру от хранилища нужен только
// GetChatByID (валидация), а декоратору — только AddMessage. Остальное — пустышки.
// ============================================================================

type fakeStorage struct {
	known    map[string]bool
	addErr   error
	addCalls int
}

var _ storage.Storage = (*fakeStorage)(nil)

func newFakeStorage(ids ...string) *fakeStorage {
	m := make(map[string]bool, len(ids))
	for _, id := range ids {
		m[id] = true
	}
	return &fakeStorage{known: m}
}

func (f *fakeStorage) GetChats() ([]domain.Chat, error) { return nil, nil }

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

func (f *fakeStorage) GetMessages() ([]domain.Message, error) { return nil, nil }

func (f *fakeStorage) GetMessagesByChatID(_ string) ([]domain.Message, error) {
	return nil, nil
}

func (f *fakeStorage) AddMessage(chatID, author, text string) (domain.Message, error) {
	if f.addErr != nil {
		return domain.Message{}, f.addErr
	}
	if !f.known[chatID] {
		return domain.Message{}, storage.ErrChatNotFound
	}
	f.addCalls++
	return domain.Message{
		ID:        "fake-msg",
		ChatID:    chatID,
		Author:    author,
		Text:      text,
		CreatedAt: "2026-10-01T00:00:00Z",
	}, nil
}
