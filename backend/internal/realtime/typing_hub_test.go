package realtime_test

import (
	"testing"

	"myaukalo/backend/internal/realtime"
)

func TestTypingHub_PublishSubscribe(t *testing.T) {
	hub := realtime.NewHub()
	defer hub.Close()

	ch := hub.SubscribeTyping("chat-1")
	if got := hub.TypingSubscriberCount("chat-1"); got != 1 {
		t.Fatalf("expected 1 typing subscriber, got %d", got)
	}

	hub.PublishTyping("chat-1", "alice")

	select {
	case user := <-ch:
		if user != "alice" {
			t.Fatalf("expected alice, got %q", user)
		}
	default:
		t.Fatal("expected typing event, got none")
	}

	hub.UnsubscribeTyping("chat-1", ch)
	if got := hub.TypingSubscriberCount("chat-1"); got != 0 {
		t.Fatalf("expected 0 after unsubscribe, got %d", got)
	}
}

func TestTypingHub_PublishNonBlocking(t *testing.T) {
	hub := realtime.NewHub()
	defer hub.Close()

	ch := hub.SubscribeTyping("chat-1")
	defer hub.UnsubscribeTyping("chat-1", ch)

	for i := 0; i < 40; i++ {
		hub.PublishTyping("chat-1", "alice")
	}
}
