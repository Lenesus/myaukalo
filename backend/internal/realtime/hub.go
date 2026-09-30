package realtime

import (
	"myaukalo/backend/internal/domain"
	"sync"
)

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

func (h *Hub) Done() <-chan struct{} { return h.done }

func (h *Hub) Close() {
	h.closeOnce.Do(func() { close(h.done) })
}

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
