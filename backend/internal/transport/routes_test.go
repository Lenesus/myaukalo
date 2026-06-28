package httptransport

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"myaukalo/backend/internal/domain"
	"myaukalo/backend/internal/storage/memory"
)

func newTestHandler(store *memory.MemoryStorage) http.Handler {
	mux := http.NewServeMux()
	RegisterRoutes(mux, store)

	return mux
}

func TestCreateChat(t *testing.T) {
	store := memory.NewMemoryStorage()
	handler := newTestHandler(store)

	requestBody := `{"title":"  Кошачий клуб  "}`

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/chats",
		strings.NewReader(requestBody),
	)

	request.Header.Set("Content-Type", "application/json")

	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusCreated,
			response.Code,
		)
	}

	var createdChat domain.Chat

	if err := json.NewDecoder(response.Body).Decode(&createdChat); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if createdChat.Title != "Кошачий клуб" {
		t.Errorf(
			"expected title %q, got %q",
			"Кошачий клуб",
			createdChat.Title,
		)
	}

	if createdChat.ID == "" {
		t.Error("expected created chat to have an id")
	}

	storedChat, found := store.GetChatByID(createdChat.ID)
	if !found {
		t.Fatalf("created chat %q was not found in storage", createdChat.ID)
	}

	if storedChat.Title != createdChat.Title {
		t.Errorf(
			"expected stored title %q, got %q",
			createdChat.Title,
			storedChat.Title,
		)
	}
}

func TestUnknownChatReturnsNotFound(t *testing.T) {
	store := memory.NewMemoryStorage()
	handler := newTestHandler(store)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/chats/no-such-chat/messages",
		nil,
	)

	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusNotFound,
			response.Code,
		)
	}

	var responseBody map[string]string

	if err := json.NewDecoder(response.Body).Decode(&responseBody); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if responseBody["error"] != "chat not found" {
		t.Errorf(
			"expected error %q, got %q",
			"chat not found",
			responseBody["error"],
		)
	}
}

func TestCreateMessageUpdatesChatPreview(t *testing.T) {
	store := memory.NewMemoryStorage()
	handler := newTestHandler(store)

	requestBody := `{
		"author":"  Барсик  ",
		"text":"  Я нашёл новую коробку  "
	}`

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/chats/chat-1/messages",
		strings.NewReader(requestBody),
	)

	request.Header.Set("Content-Type", "application/json")

	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusCreated,
			response.Code,
		)
	}

	var createdMessage domain.Message

	if err := json.NewDecoder(response.Body).Decode(&createdMessage); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if createdMessage.Author != "Барсик" {
		t.Errorf(
			"expected author %q, got %q",
			"Барсик",
			createdMessage.Author,
		)
	}

	if createdMessage.Text != "Я нашёл новую коробку" {
		t.Errorf(
			"expected text %q, got %q",
			"Я нашёл новую коробку",
			createdMessage.Text,
		)
	}

	chat, found := store.GetChatByID("chat-1")
	if !found {
		t.Fatal("expected chat-1 to exist")
	}

	if chat.LastMessage != createdMessage.Text {
		t.Errorf(
			"expected last message %q, got %q",
			createdMessage.Text,
			chat.LastMessage,
		)
	}
}
