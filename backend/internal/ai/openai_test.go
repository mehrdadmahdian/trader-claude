package ai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenAIChat_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			t.Error("expected Authorization header")
		}
		w.WriteHeader(http.StatusOK)
		// Response JSON with proper escape sequences
		w.Write([]byte("{\"choices\":[{\"message\":{\"content\":\"RSI divergence means...\\n\\n```suggestions\\n[\\\"What about MACD?\\\",\\\"Best timeframe?\\\",\\\"How to confirm?\\\"]\\n```\"}}]}"))
	}))
	defer server.Close()

	old := openaiBaseURL
	openaiBaseURL = server.URL
	defer func() { openaiBaseURL = old }()

	provider := NewOpenAIProvider("test-key", "gpt-4o-mini")
	resp, err := provider.Chat(context.Background(), []ChatMessage{
		{Role: RoleUser, Content: "What is RSI divergence?"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Reply == "" {
		t.Error("expected non-empty reply")
	}
	if len(resp.SuggestedQuestions) != 3 {
		t.Errorf("expected 3 suggestions, got %d", len(resp.SuggestedQuestions))
	}
}

func TestOpenAIChat_Unauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"invalid key"}`))
	}))
	defer server.Close()

	old := openaiBaseURL
	openaiBaseURL = server.URL
	defer func() { openaiBaseURL = old }()

	provider := NewOpenAIProvider("bad-key", "")
	_, err := provider.Chat(context.Background(), []ChatMessage{
		{Role: RoleUser, Content: "test"},
	})
	if err == nil {
		t.Fatal("expected error for unauthorized")
	}
}

func TestParseSuggestions_WithBlock(t *testing.T) {
	content := "Here is my answer.\n\n" + "```suggestions\n" + `["Q1","Q2","Q3"]` + "\n```"
	reply, suggestions := parseSuggestions(content)
	if reply != "Here is my answer." {
		t.Errorf("unexpected reply: %s", reply)
	}
	if len(suggestions) != 3 {
		t.Errorf("expected 3, got %d", len(suggestions))
	}
}

func TestParseSuggestions_WithoutBlock(t *testing.T) {
	content := "Here is my answer without suggestions."
	reply, suggestions := parseSuggestions(content)
	if reply != content {
		t.Errorf("unexpected reply: %s", reply)
	}
	if len(suggestions) != 3 {
		t.Errorf("expected 3 defaults, got %d", len(suggestions))
	}
}

func TestOpenAITestConnection_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data":[]}`))
	}))
	defer server.Close()

	old := openaiBaseURL
	openaiBaseURL = server.URL
	defer func() { openaiBaseURL = old }()

	provider := NewOpenAIProvider("test-key", "")
	err := provider.TestConnection(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
