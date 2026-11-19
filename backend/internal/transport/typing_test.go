package httptransport_test

import (
	nethttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"myaukalo/backend/internal/storage/memory"
	httptransport "myaukalo/backend/internal/transport"
)

func newTypingMux() *nethttp.ServeMux {
	store := memory.NewMemoryStorage()
	mux := nethttp.NewServeMux()
	httptransport.RegisterRoutes(mux, store)
	return mux
}

func TestTypingRoute_OK(t *testing.T) {
	mux := newTypingMux()

	req := httptest.NewRequest(nethttp.MethodPost, "/api/v1/chats/chat-1/typing", strings.NewReader(`{"user":"alice"}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != nethttp.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestTypingRoute_UnknownChat(t *testing.T) {
	mux := newTypingMux()

	req := httptest.NewRequest(nethttp.MethodPost, "/api/v1/chats/chat-nope/typing", strings.NewReader(`{"user":"alice"}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != nethttp.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestTypingRoute_BadBody(t *testing.T) {
	mux := newTypingMux()

	req := httptest.NewRequest(nethttp.MethodPost, "/api/v1/chats/chat-1/typing", strings.NewReader(`{not json`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != nethttp.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestTypingRoute_MissingUser(t *testing.T) {
	mux := newTypingMux()

	req := httptest.NewRequest(nethttp.MethodPost, "/api/v1/chats/chat-1/typing", strings.NewReader(`{"user":""}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != nethttp.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestTypingRoute_MethodNotAllowed(t *testing.T) {
	mux := newTypingMux()

	req := httptest.NewRequest(nethttp.MethodGet, "/api/v1/chats/chat-1/typing", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != nethttp.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
}
