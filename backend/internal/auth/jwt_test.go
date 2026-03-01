package auth

import (
	"testing"
	"time"
)

var testSecret = []byte("test-jwt-secret-key-for-unit-tests")

func TestGenerateAndParse_Success(t *testing.T) {
	token, err := GenerateAccessToken(testSecret, 42, "user@test.com", "user", "John", 15*time.Minute)
	if err != nil {
		t.Fatalf("generate error: %v", err)
	}

	claims, err := ParseAccessToken(testSecret, token)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if claims.UserID != 42 {
		t.Errorf("expected UserID 42, got %d", claims.UserID)
	}
	if claims.Email != "user@test.com" {
		t.Errorf("expected email user@test.com, got %s", claims.Email)
	}
	if claims.Role != "user" {
		t.Errorf("expected role user, got %s", claims.Role)
	}
}

func TestParse_ExpiredToken(t *testing.T) {
	token, err := GenerateAccessToken(testSecret, 1, "a@b.com", "user", "A", -1*time.Hour)
	if err != nil {
		t.Fatalf("generate error: %v", err)
	}

	_, err = ParseAccessToken(testSecret, token)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestParse_WrongSecret(t *testing.T) {
	token, err := GenerateAccessToken(testSecret, 1, "a@b.com", "user", "A", 15*time.Minute)
	if err != nil {
		t.Fatalf("generate error: %v", err)
	}

	_, err = ParseAccessToken([]byte("wrong-secret"), token)
	if err == nil {
		t.Fatal("expected error for wrong secret")
	}
}

func TestParse_MalformedToken(t *testing.T) {
	_, err := ParseAccessToken(testSecret, "not-a-valid-jwt")
	if err == nil {
		t.Fatal("expected error for malformed token")
	}
}
