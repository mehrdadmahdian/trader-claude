package notification

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSendText_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	old := telegramAPIURL
	telegramAPIURL = server.URL + "/bot"
	defer func() { telegramAPIURL = old }()

	sender := NewTelegramSender("test-token")
	err := sender.SendText(context.Background(), "123", "Hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSendText_Failure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"ok":false,"description":"Unauthorized"}`))
	}))
	defer server.Close()

	old := telegramAPIURL
	telegramAPIURL = server.URL + "/bot"
	defer func() { telegramAPIURL = old }()

	sender := NewTelegramSender("bad-token")
	err := sender.SendText(context.Background(), "123", "Hello")
	if err == nil {
		t.Fatal("expected error for unauthorized")
	}
}

func TestSendPhoto_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ct := r.Header.Get("Content-Type")
		if ct == "" {
			t.Error("expected Content-Type header")
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	old := telegramAPIURL
	telegramAPIURL = server.URL + "/bot"
	defer func() { telegramAPIURL = old }()

	sender := NewTelegramSender("test-token")
	err := sender.SendPhoto(context.Background(), "123", []byte{0x89, 0x50, 0x4e, 0x47}, "test caption")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTestConnection_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true,"result":{"username":"test_bot"}}`))
	}))
	defer server.Close()

	old := telegramAPIURL
	telegramAPIURL = server.URL + "/bot"
	defer func() { telegramAPIURL = old }()

	sender := NewTelegramSender("test-token")
	name, err := sender.TestConnection(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "test_bot" {
		t.Errorf("expected test_bot, got %s", name)
	}
}
