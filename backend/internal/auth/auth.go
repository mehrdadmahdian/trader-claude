package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/trader-claude/backend/internal/models"
)

type AuthService struct {
	db         *gorm.DB
	jwtSecret  []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
}

func NewAuthService(db *gorm.DB, jwtSecret string) *AuthService {
	return &AuthService{
		db:         db,
		jwtSecret:  []byte(jwtSecret),
		accessTTL:  15 * time.Minute,
		refreshTTL: 7 * 24 * time.Hour,
	}
}

func (s *AuthService) Register(ctx context.Context, email, password, displayName string) (*models.User, string, error) {
	if err := ValidatePasswordPolicy(password); err != nil {
		return nil, "", fmt.Errorf("password policy: %w", err)
	}

	hash, err := HashPassword(password)
	if err != nil {
		return nil, "", err
	}

	// First user becomes admin
	var count int64
	s.db.WithContext(ctx).Model(&models.User{}).Count(&count)
	role := models.UserRoleUser
	if count == 0 {
		role = models.UserRoleAdmin
	}

	user := &models.User{
		Email:        email,
		PasswordHash: hash,
		DisplayName:  displayName,
		Role:         role,
		Active:       true,
	}

	if err := s.db.WithContext(ctx).Create(user).Error; err != nil {
		return nil, "", fmt.Errorf("create user: %w", err)
	}

	accessToken, err := GenerateAccessToken(s.jwtSecret, user.ID, user.Email, string(user.Role), user.DisplayName, s.accessTTL)
	if err != nil {
		return nil, "", err
	}

	return user, accessToken, nil
}

func (s *AuthService) Login(ctx context.Context, email, password, userAgent, ip string) (string, string, *models.User, error) {
	var user models.User
	if err := s.db.WithContext(ctx).Where("email = ?", email).First(&user).Error; err != nil {
		return "", "", nil, fmt.Errorf("invalid email or password")
	}

	if !user.Active {
		return "", "", nil, fmt.Errorf("account is disabled")
	}

	if err := ComparePassword(user.PasswordHash, password); err != nil {
		return "", "", nil, fmt.Errorf("invalid email or password")
	}

	accessToken, err := GenerateAccessToken(s.jwtSecret, user.ID, user.Email, string(user.Role), user.DisplayName, s.accessTTL)
	if err != nil {
		return "", "", nil, err
	}

	refreshToken, err := s.createRefreshToken(ctx, user.ID, userAgent, ip)
	if err != nil {
		return "", "", nil, err
	}

	now := time.Now()
	s.db.WithContext(ctx).Model(&user).Update("last_login_at", &now)

	return accessToken, refreshToken, &user, nil
}

func (s *AuthService) RefreshToken(ctx context.Context, oldToken, userAgent, ip string) (string, string, error) {
	tokenHash := hashToken(oldToken)

	var rt models.RefreshToken
	if err := s.db.WithContext(ctx).Where("token_hash = ? AND revoked = false", tokenHash).First(&rt).Error; err != nil {
		return "", "", fmt.Errorf("invalid refresh token")
	}

	if time.Now().After(rt.ExpiresAt) {
		s.db.WithContext(ctx).Model(&rt).Update("revoked", true)
		return "", "", fmt.Errorf("refresh token expired")
	}

	// Revoke old token (rotation)
	s.db.WithContext(ctx).Model(&rt).Update("revoked", true)

	var user models.User
	if err := s.db.WithContext(ctx).First(&user, rt.UserID).Error; err != nil {
		return "", "", fmt.Errorf("user not found")
	}

	if !user.Active {
		return "", "", fmt.Errorf("account is disabled")
	}

	accessToken, err := GenerateAccessToken(s.jwtSecret, user.ID, user.Email, string(user.Role), user.DisplayName, s.accessTTL)
	if err != nil {
		return "", "", err
	}

	newRefresh, err := s.createRefreshToken(ctx, user.ID, userAgent, ip)
	if err != nil {
		return "", "", err
	}

	return accessToken, newRefresh, nil
}

func (s *AuthService) Logout(ctx context.Context, userID int64) error {
	return s.db.WithContext(ctx).
		Model(&models.RefreshToken{}).
		Where("user_id = ? AND revoked = false", userID).
		Update("revoked", true).Error
}

func (s *AuthService) ValidateAccessToken(tokenStr string) (*Claims, error) {
	return ParseAccessToken(s.jwtSecret, tokenStr)
}

// GetUser fetches a user by ID from the database.
func (s *AuthService) GetUser(ctx context.Context, userID int64) (*models.User, error) {
	var user models.User
	if err := s.db.WithContext(ctx).First(&user, userID).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// UpdateProfile updates a user's display name and/or password.
// If password is non-empty, currentPassword must be correct.
func (s *AuthService) UpdateProfile(ctx context.Context, userID int64, displayName, password, currentPassword string) (*models.User, error) {
	var user models.User
	if err := s.db.WithContext(ctx).First(&user, userID).Error; err != nil {
		return nil, fmt.Errorf("user not found")
	}

	if password != "" {
		if err := ComparePassword(user.PasswordHash, currentPassword); err != nil {
			return nil, fmt.Errorf("current password is incorrect")
		}
		if err := ValidatePasswordPolicy(password); err != nil {
			return nil, err
		}
		hash, err := HashPassword(password)
		if err != nil {
			return nil, fmt.Errorf("failed to hash password")
		}
		user.PasswordHash = hash
	}

	if displayName != "" {
		user.DisplayName = displayName
	}

	if err := s.db.WithContext(ctx).Save(&user).Error; err != nil {
		return nil, fmt.Errorf("failed to update profile")
	}
	return &user, nil
}

func (s *AuthService) createRefreshToken(ctx context.Context, userID int64, userAgent, ip string) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate refresh token: %w", err)
	}
	tokenStr := hex.EncodeToString(raw)

	rt := &models.RefreshToken{
		UserID:    userID,
		TokenHash: hashToken(tokenStr),
		ExpiresAt: time.Now().Add(s.refreshTTL),
		UserAgent: userAgent,
		IP:        ip,
	}

	if err := s.db.WithContext(ctx).Create(rt).Error; err != nil {
		return "", fmt.Errorf("save refresh token: %w", err)
	}

	return tokenStr, nil
}

func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
