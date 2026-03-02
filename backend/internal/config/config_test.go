package config

import (
	"testing"
)

func TestValidate_ProductionMissingJWT(t *testing.T) {
	cfg := &Config{App: AppConfig{Env: "production", JWTSecret: ""}}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for missing JWT secret")
	}
}

func TestValidate_ProductionWeakJWT(t *testing.T) {
	cfg := &Config{App: AppConfig{Env: "production", JWTSecret: "short"}}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for short JWT secret")
	}
}

func TestValidate_ProductionDefaultPassword(t *testing.T) {
	cfg := &Config{App: AppConfig{Env: "production", JWTSecret: "this-is-a-32-char-secret-for-test!"},
		DB:   DBConfig{Password: "traderpassword"},
		CORS: CORSConfig{Origins: "https://example.com"}}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for default DB password")
	}
}

func TestValidate_DevelopmentNoErrors(t *testing.T) {
	cfg := &Config{App: AppConfig{Env: "development"}}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("unexpected error for development: %v", err)
	}
}

func TestValidate_ProductionValid(t *testing.T) {
	cfg := &Config{
		App:  AppConfig{Env: "production", JWTSecret: "this-is-a-32-char-secret-for-test!"},
		DB:   DBConfig{Password: "secure-prod-pass"},
		CORS: CORSConfig{Origins: "https://example.com"},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("unexpected error for valid production config: %v", err)
	}
}
