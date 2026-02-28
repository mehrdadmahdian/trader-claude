package ai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOllamaChat_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message":{"content":"Ollama says hello"}}`))
	}))
	defer server.Close()

	provider := NewOllamaProvider(server.URL, "llama3.2")
	resp, err := provider.Chat(context.Background(), []ChatMessage{
		{Role: RoleUser, Content: "Hello"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Reply != "Ollama says hello" {
		t.Errorf("unexpected reply: %s", resp.Reply)
	}
}

func TestOllamaTestConnection_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"models":[]}`))
	}))
	defer server.Close()

	provider := NewOllamaProvider(server.URL, "")
	err := provider.TestConnection(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOllamaTestConnection_Failure(t *testing.T) {
	provider := NewOllamaProvider("http://localhost:1", "")
	err := provider.TestConnection(context.Background())
	if err == nil {
		t.Fatal("expected error for unreachable server")
	}
}
