package httptransport

import (
	"encoding/json"
	nethttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"myaukalo/backend/internal/domain"
	"myaukalo/backend/internal/storage/memory"
)

func newReactionsMux() *nethttp.ServeMux {
	store := memory.NewMemoryStorage()
	mux := nethttp.NewServeMux()
	RegisterRoutes(mux, store)
	return mux
}

func TestReactionsRoute_AddAndRemove(t *testing.T) {
	mux := newReactionsMux()
	body := `{"user":"alice","emoji":"👍"}`
	path := "/api/v1/chats/chat-1/messages/msg-1/reactions"

	req := httptest.NewRequest(nethttp.MethodPost, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != nethttp.StatusOK {
		t.Fatalf("POST expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var msg domain.Message
	if err := json.Unmarshal(rec.Body.Bytes(), &msg); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(msg.Reactions) != 1 {
		t.Fatalf("expected one reaction, got %+v", msg.Reactions)
	}

	req = httptest.NewRequest(nethttp.MethodDelete, path, strings.NewReader(body))
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != nethttp.StatusOK {
		t.Fatalf("DELETE expected 200, got %d", rec.Code)
	}
	msg = domain.Message{}
	if err := json.Unmarshal(rec.Body.Bytes(), &msg); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(msg.Reactions) != 0 {
		t.Fatalf("expected no reactions after remove, got %+v", msg.Reactions)
	}
}

func TestReactionsRoute_UnknownMessage(t *testing.T) {
	mux := newReactionsMux()

	req := httptest.NewRequest(
		nethttp.MethodPost,
		"/api/v1/chats/chat-1/messages/msg-nope/reactions",
		strings.NewReader(`{"user":"a","emoji":"👍"}`),
	)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != nethttp.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestReactionsRoute_WrongChat(t *testing.T) {
	mux := newReactionsMux()

	// msg-1 принадлежит chat-1, не chat-2
	req := httptest.NewRequest(
		nethttp.MethodPost,
		"/api/v1/chats/chat-2/messages/msg-1/reactions",
		strings.NewReader(`{"user":"a","emoji":"👍"}`),
	)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != nethttp.StatusNotFound {
		t.Fatalf("expected 404 for wrong chat, got %d", rec.Code)
	}
}

func TestReactionsRoute_BadBody(t *testing.T) {
	mux := newReactionsMux()

	req := httptest.NewRequest(
		nethttp.MethodPost,
		"/api/v1/chats/chat-1/messages/msg-1/reactions",
		strings.NewReader(`{not json`),
	)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != nethttp.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestReactionsRoute_MethodNotAllowed(t *testing.T) {
	mux := newReactionsMux()

	req := httptest.NewRequest(
		nethttp.MethodGet,
		"/api/v1/chats/chat-1/messages/msg-1/reactions",
		nil,
	)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != nethttp.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
}
