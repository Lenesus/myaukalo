package realtime

import (
	"sync"

	"myaukalo/backend/internal/domain"
)

// Hub — реестр подписчиков на новые сообщения, сгруппированных по чатам.
type Hub struct {
	mu   sync.Mutex
	subs map[string]map[chan domain.Message]struct{}

	done      chan struct{}
	closeOnce sync.Once
}

func NewHub() *Hub {
	return &Hub{
		subs: make(map[string]map[chan domain.Message]struct{}),
		done: make(chan struct{}),
	}
}

// Done возвращает канал, который закроется при остановке хаба.
func (h *Hub) Done() <-chan struct{} { return h.done }

// Close идемпотентно останавливает хаб. Повторные вызовы безопасны.
func (h *Hub) Close() {
	h.closeOnce.Do(func() { close(h.done) })
}

// SubscriberCount возвращает число активных подписчиков чата.
// Штатная наблюдаемость: в проде такие счётчики идут в метрики и отладку.
func (h *Hub) SubscriberCount(chatID string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.subs[chatID])
}

// Subscribe регистрирует нового зрителя чата и возвращает его личный канал.
func (h *Hub) Subscribe(chatID string) chan domain.Message {
	h.mu.Lock()
	defer h.mu.Unlock()

	ch := make(chan domain.Message, 16)

	set := h.subs[chatID]
	if set == nil {
		set = make(map[chan domain.Message]struct{})
		h.subs[chatID] = set
	}
	set[ch] = struct{}{}

	return ch
}

// Unsubscribe удаляет зрителя из чата. Канал намеренно НЕ закрывается:
// запись в закрытый канал — panic, а гонка Publish/Unsubscribe воспроизводима
// под нагрузкой. Просто перестаём слать, GC заберёт канал когда читатель выйдет.
func (h *Hub) Unsubscribe(chatID string, ch chan domain.Message) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if set, ok := h.subs[chatID]; ok {
		delete(set, ch)
		if len(set) == 0 {
			delete(h.subs, chatID)
		}
	}
}

// Publish рассылает сообщение зрителям его чата (неблокирующе).
func (h *Hub) Publish(msg domain.Message) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for ch := range h.subs[msg.ChatID] {
		select {
		case ch <- msg:
		default:
		}
	}
}
