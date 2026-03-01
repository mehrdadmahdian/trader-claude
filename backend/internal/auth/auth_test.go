package auth

import (
	"context"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/trader-claude/backend/internal/models"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.RefreshToken{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestRegister_Success(t *testing.T) {
	db := setupTestDB(t)
	svc := NewAuthService(db, "test-secret")

	user, token, err := svc.Register(context.Background(), "user@test.com", "SecurePass1", "Test User")
	if err != nil {
		t.Fatalf("register error: %v", err)
	}
	if user.Email != "user@test.com" {
		t.Errorf("expected user@test.com, got %s", user.Email)
	}
	if token == "" {
		t.Error("expected non-empty access token")
	}
	// First user should be admin
	if user.Role != models.UserRoleAdmin {
		t.Errorf("expected first user to be admin, got %s", user.Role)
	}
}

func TestRegister_SecondUserIsRegular(t *testing.T) {
	db := setupTestDB(t)
	svc := NewAuthService(db, "test-secret")

	svc.Register(context.Background(), "admin@test.com", "SecurePass1", "Admin")
	user, _, err := svc.Register(context.Background(), "user@test.com", "SecurePass1", "User")
	if err != nil {
		t.Fatalf("register error: %v", err)
	}
	if user.Role != models.UserRoleUser {
		t.Errorf("expected second user to be regular user, got %s", user.Role)
	}
}

func TestRegister_DuplicateEmail(t *testing.T) {
	db := setupTestDB(t)
	svc := NewAuthService(db, "test-secret")

	svc.Register(context.Background(), "dup@test.com", "SecurePass1", "First")
	_, _, err := svc.Register(context.Background(), "dup@test.com", "SecurePass1", "Second")
	if err == nil {
		t.Fatal("expected error for duplicate email")
	}
}

func TestRegister_BadPassword(t *testing.T) {
	db := setupTestDB(t)
	svc := NewAuthService(db, "test-secret")

	_, _, err := svc.Register(context.Background(), "user@test.com", "short", "User")
	if err == nil {
		t.Fatal("expected error for bad password")
	}
}

func TestLogin_Success(t *testing.T) {
	db := setupTestDB(t)
	svc := NewAuthService(db, "test-secret")

	svc.Register(context.Background(), "login@test.com", "SecurePass1", "User")

	access, refresh, user, err := svc.Login(context.Background(), "login@test.com", "SecurePass1", "test-agent", "127.0.0.1")
	if err != nil {
		t.Fatalf("login error: %v", err)
	}
	if access == "" || refresh == "" {
		t.Error("expected non-empty tokens")
	}
	if user.Email != "login@test.com" {
		t.Errorf("expected login@test.com, got %s", user.Email)
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	db := setupTestDB(t)
	svc := NewAuthService(db, "test-secret")

	svc.Register(context.Background(), "user@test.com", "SecurePass1", "User")

	_, _, _, err := svc.Login(context.Background(), "user@test.com", "WrongPass1", "agent", "127.0.0.1")
	if err == nil {
		t.Fatal("expected error for wrong password")
	}
}

func TestLogin_NonExistentUser(t *testing.T) {
	db := setupTestDB(t)
	svc := NewAuthService(db, "test-secret")

	_, _, _, err := svc.Login(context.Background(), "nobody@test.com", "SecurePass1", "agent", "127.0.0.1")
	if err == nil {
		t.Fatal("expected error for non-existent user")
	}
}

func TestRefreshToken_Success(t *testing.T) {
	db := setupTestDB(t)
	svc := NewAuthService(db, "test-secret")

	svc.Register(context.Background(), "user@test.com", "SecurePass1", "User")
	_, refresh, _, _ := svc.Login(context.Background(), "user@test.com", "SecurePass1", "agent", "127.0.0.1")

	newAccess, newRefresh, err := svc.RefreshToken(context.Background(), refresh, "agent", "127.0.0.1")
	if err != nil {
		t.Fatalf("refresh error: %v", err)
	}
	if newAccess == "" || newRefresh == "" {
		t.Error("expected non-empty tokens after refresh")
	}
	// Old token should be revoked
	_, _, err = svc.RefreshToken(context.Background(), refresh, "agent", "127.0.0.1")
	if err == nil {
		t.Fatal("expected error for reused refresh token")
	}
}

func TestLogout_RevokesAllTokens(t *testing.T) {
	db := setupTestDB(t)
	svc := NewAuthService(db, "test-secret")

	svc.Register(context.Background(), "user@test.com", "SecurePass1", "User")
	_, refresh, user, _ := svc.Login(context.Background(), "user@test.com", "SecurePass1", "agent", "127.0.0.1")

	if err := svc.Logout(context.Background(), user.ID); err != nil {
		t.Fatalf("logout error: %v", err)
	}

	_, _, err := svc.RefreshToken(context.Background(), refresh, "agent", "127.0.0.1")
	if err == nil {
		t.Fatal("expected error after logout")
	}
}
