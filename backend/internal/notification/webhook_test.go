package notification

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestWebhookSend_Success(t *testing.T) {
	var gotSig string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSig = r.Header.Get("X-TraderClaude-Signature")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ws := NewWebhookSender()
	payload := []byte(`{"event":"test"}`)
	err := ws.Send(context.Background(), server.URL, "my-secret", payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotSig == "" {
		t.Error("expected signature header")
	}
	if !VerifySignature("my-secret", payload, gotSig) {
		t.Error("signature verification failed")
	}
}

func TestWebhookSend_RetryOn500(t *testing.T) {
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&attempts, 1)
		if n < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ws := NewWebhookSender()
	err := ws.Send(context.Background(), server.URL, "secret", []byte(`{}`))
	if err != nil {
		t.Fatalf("expected success after retries, got: %v", err)
	}
	if atomic.LoadInt32(&attempts) != 3 {
		t.Errorf("expected 3 attempts, got %d", atomic.LoadInt32(&attempts))
	}
}

func TestWebhookSend_NoRetryOn4xx(t *testing.T) {
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	ws := NewWebhookSender()
	err := ws.Send(context.Background(), server.URL, "secret", []byte(`{}`))
	if err == nil {
		t.Fatal("expected error for 400")
	}
	if atomic.LoadInt32(&attempts) != 1 {
		t.Errorf("expected 1 attempt (no retry on 4xx), got %d", atomic.LoadInt32(&attempts))
	}
}

func TestVerifySignature(t *testing.T) {
	payload := []byte(`{"test":true}`)
	sig := "sha256=" + computeHMAC("secret", payload)
	if !VerifySignature("secret", payload, sig) {
		t.Error("expected signature to verify")
	}
	if VerifySignature("wrong-secret", payload, sig) {
		t.Error("expected signature to fail with wrong secret")
	}
}
