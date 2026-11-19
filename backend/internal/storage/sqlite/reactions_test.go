package sqlite

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"myaukalo/backend/internal/storage"
)

func newReactionsTestStore(t *testing.T) *Storage {
	t.Helper()

	s, err := New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	return s
}

func TestReactions_AddAndAggregate(t *testing.T) {
	s := newReactionsTestStore(t)

	chat, err := s.AddChat("room")
	if err != nil {
		t.Fatalf("add chat: %v", err)
	}

	msg, err := s.AddMessage(chat.ID, "alice", "hi", 0)
	if err != nil {
		t.Fatalf("add message: %v", err)
	}

	updated, err := s.AddReaction(msg.ID, "alice", "👍")
	if err != nil {
		t.Fatalf("add reaction: %v", err)
	}
	if len(updated.Reactions) != 1 || updated.Reactions[0].Emoji != "👍" {
		t.Fatalf("expected one 👍 reaction, got %+v", updated.Reactions)
	}
	if len(updated.Reactions[0].Users) != 1 || updated.Reactions[0].Users[0] != "alice" {
		t.Fatalf("expected users [alice], got %+v", updated.Reactions[0].Users)
	}

	updated, err = s.AddReaction(msg.ID, "bob", "👍")
	if err != nil {
		t.Fatalf("add second user: %v", err)
	}
	if len(updated.Reactions[0].Users) != 2 {
		t.Fatalf("expected two users, got %+v", updated.Reactions[0].Users)
	}

	// идемпотентность: повтор alice не дублирует
	updated, err = s.AddReaction(msg.ID, "alice", "👍")
	if err != nil {
		t.Fatalf("repeat add: %v", err)
	}
	if len(updated.Reactions[0].Users) != 2 {
		t.Fatalf("duplicate add must not duplicate user, got %+v", updated.Reactions[0].Users)
	}

	// второй эмодзи
	updated, err = s.AddReaction(msg.ID, "alice", "❤️")
	if err != nil {
		t.Fatalf("add second emoji: %v", err)
	}
	if len(updated.Reactions) != 2 {
		t.Fatalf("expected two emoji groups, got %+v", updated.Reactions)
	}

	// read-путь прикрепляет реакции
	msgs, err := s.GetMessagesByChatID(chat.ID)
	if err != nil {
		t.Fatalf("get messages: %v", err)
	}
	var found bool
	for _, m := range msgs {
		if m.ID == msg.ID {
			found = true
			if len(m.Reactions) != 2 {
				t.Fatalf("read path must attach reactions, got %+v", m.Reactions)
			}
		}
	}
	if !found {
		t.Fatalf("message not found in read path")
	}
}

func TestReactions_Remove(t *testing.T) {
	s := newReactionsTestStore(t)

	chat, _ := s.AddChat("room")
	msg, _ := s.AddMessage(chat.ID, "alice", "hi", 0)

	_, _ = s.AddReaction(msg.ID, "alice", "👍")
	_, _ = s.AddReaction(msg.ID, "bob", "👍")

	updated, err := s.RemoveReaction(msg.ID, "alice", "👍")
	if err != nil {
		t.Fatalf("remove: %v", err)
	}
	if len(updated.Reactions) != 1 || len(updated.Reactions[0].Users) != 1 || updated.Reactions[0].Users[0] != "bob" {
		t.Fatalf("expected only bob left, got %+v", updated.Reactions)
	}

	updated, err = s.RemoveReaction(msg.ID, "bob", "👍")
	if err != nil {
		t.Fatalf("remove last: %v", err)
	}
	if len(updated.Reactions) != 0 {
		t.Fatalf("expected no reactions, got %+v", updated.Reactions)
	}
}

func TestReactions_UnknownMessage(t *testing.T) {
	s := newReactionsTestStore(t)

	_, err := s.AddReaction("msg-nope", "alice", "👍")
	if !errors.Is(err, storage.ErrMessageNotFound) {
		t.Fatalf("expected ErrMessageNotFound, got %v", err)
	}
}

func TestReactions_CascadeOnExpiry(t *testing.T) {
	s := newReactionsTestStore(t)

	chat, _ := s.AddChat("room")
	msg, err := s.AddMessage(chat.ID, "alice", "ephemeral", time.Second)
	if err != nil {
		t.Fatalf("add ttl message: %v", err)
	}

	if _, err := s.AddReaction(msg.ID, "alice", "👍"); err != nil {
		t.Fatalf("add reaction: %v", err)
	}

	removed, err := s.DeleteExpiredMessages(time.Now().UTC().Add(2 * time.Second))
	if err != nil {
		t.Fatalf("delete expired: %v", err)
	}
	if removed != 1 {
		t.Fatalf("expected 1 removed, got %d", removed)
	}

	var count int
	if err := s.db.QueryRow(
		`SELECT COUNT(*) FROM reactions WHERE message_id = ?`, msg.ID,
	).Scan(&count); err != nil {
		t.Fatalf("count reactions: %v", err)
	}
	if count != 0 {
		t.Fatalf("FK cascade must remove reactions, got %d", count)
	}
}
