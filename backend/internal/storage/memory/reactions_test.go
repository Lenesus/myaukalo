package memory

import (
	"errors"
	"testing"
	"time"

	storagecontract "myaukalo/backend/internal/storage"
)

func TestMemoryReactions_AddRemove(t *testing.T) {
	s := NewMemoryStorage()

	updated, err := s.AddReaction("msg-1", "alice", "👍")
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if len(updated.Reactions) != 1 || updated.Reactions[0].Users[0] != "alice" {
		t.Fatalf("expected alice 👍, got %+v", updated.Reactions)
	}

	updated, err = s.AddReaction("msg-1", "bob", "👍")
	if err != nil {
		t.Fatalf("add bob: %v", err)
	}
	if len(updated.Reactions[0].Users) != 2 {
		t.Fatalf("expected two users, got %+v", updated.Reactions[0].Users)
	}

	updated, err = s.RemoveReaction("msg-1", "alice", "👍")
	if err != nil {
		t.Fatalf("remove: %v", err)
	}
	if len(updated.Reactions[0].Users) != 1 || updated.Reactions[0].Users[0] != "bob" {
		t.Fatalf("expected bob left, got %+v", updated.Reactions)
	}
}

func TestMemoryReactions_ReadAttaches(t *testing.T) {
	s := NewMemoryStorage()

	if _, err := s.AddReaction("msg-1", "alice", "👍"); err != nil {
		t.Fatalf("add: %v", err)
	}

	msgs, err := s.GetMessagesByChatID("chat-1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	var found bool
	for _, m := range msgs {
		if m.ID == "msg-1" {
			found = true
			if len(m.Reactions) != 1 {
				t.Fatalf("read must attach reactions, got %+v", m.Reactions)
			}
		}
	}
	if !found {
		t.Fatalf("msg-1 not found")
	}
}

func TestMemoryReactions_Unknown(t *testing.T) {
	s := NewMemoryStorage()

	_, err := s.AddReaction("msg-nope", "alice", "👍")
	if !errors.Is(err, storagecontract.ErrMessageNotFound) {
		t.Fatalf("expected ErrMessageNotFound, got %v", err)
	}
}

func TestMemoryReactions_ExpiryCleans(t *testing.T) {
	s := NewMemoryStorage()

	msg, err := s.AddMessage("chat-1", "alice", "ephemeral", time.Second)
	if err != nil {
		t.Fatalf("add ttl: %v", err)
	}
	if _, err := s.AddReaction(msg.ID, "alice", "👍"); err != nil {
		t.Fatalf("add reaction: %v", err)
	}

	if _, err := s.DeleteExpiredMessages(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("delete expired: %v", err)
	}

	_, err = s.AddReaction(msg.ID, "alice", "👍")
	if !errors.Is(err, storagecontract.ErrMessageNotFound) {
		t.Fatalf("expired message must reject reactions, got %v", err)
	}
}
