package admin

import (
	"testing"
	"time"
)

func TestGenerateToken(t *testing.T) {
	cfg := &JWTConfig{
		SecretKey:     "test-secret-key-32-bytes-long!",
		AccessExpiry:  15 * time.Minute,
		RefreshExpiry: 7 * 24 * time.Hour,
	}
	auth := NewAuthService(cfg)

	tests := []struct {
		name      string
		adminID   string
		companyID string
		roleID    string
		shouldErr bool
	}{
		{
			name:      "valid token generation",
			adminID:   "admin-1",
			companyID: "company-1",
			roleID:    "role-1",
			shouldErr: false,
		},
		{
			name:      "missing adminID",
			adminID:   "",
			companyID: "company-1",
			roleID:    "role-1",
			shouldErr: true,
		},
		{
			name:      "missing companyID",
			adminID:   "admin-1",
			companyID: "",
			roleID:    "role-1",
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := auth.GenerateToken(tt.adminID, tt.companyID, tt.roleID)
			if (err != nil) != tt.shouldErr {
				t.Errorf("GenerateToken() error = %v, shouldErr %v", err, tt.shouldErr)
				return
			}
			if err == nil && token == "" {
				t.Error("GenerateToken() returned empty token")
			}
		})
	}
}

func TestGenerateRefreshToken(t *testing.T) {
	cfg := &JWTConfig{
		SecretKey:     "test-secret-key-32-bytes-long!",
		AccessExpiry:  15 * time.Minute,
		RefreshExpiry: 7 * 24 * time.Hour,
	}
	auth := NewAuthService(cfg)

	tests := []struct {
		name      string
		adminID   string
		companyID string
		shouldErr bool
	}{
		{
			name:      "valid refresh token",
			adminID:   "admin-1",
			companyID: "company-1",
			shouldErr: false,
		},
		{
			name:      "missing adminID",
			adminID:   "",
			companyID: "company-1",
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := auth.GenerateRefreshToken(tt.adminID, tt.companyID)
			if (err != nil) != tt.shouldErr {
				t.Errorf("GenerateRefreshToken() error = %v, shouldErr %v", err, tt.shouldErr)
				return
			}
			if err == nil && token == "" {
				t.Error("GenerateRefreshToken() returned empty token")
			}
		})
	}
}

func TestValidateToken(t *testing.T) {
	cfg := &JWTConfig{
		SecretKey:     "test-secret-key-32-bytes-long!",
		AccessExpiry:  15 * time.Minute,
		RefreshExpiry: 7 * 24 * time.Hour,
	}
	auth := NewAuthService(cfg)

	// Generate a valid token
	validToken, _ := auth.GenerateToken("admin-1", "company-1", "role-1")

	tests := []struct {
		name        string
		token       string
		shouldErr   bool
		expectAdmin string
	}{
		{
			name:        "valid token",
			token:       validToken,
			shouldErr:   false,
			expectAdmin: "admin-1",
		},
		{
			name:      "invalid token",
			token:     "invalid.token.here",
			shouldErr: true,
		},
		{
			name:      "empty token",
			token:     "",
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims, err := auth.ValidateToken(tt.token)
			if (err != nil) != tt.shouldErr {
				t.Errorf("ValidateToken() error = %v, shouldErr %v", err, tt.shouldErr)
				return
			}
			if err == nil && claims.AdminID != tt.expectAdmin {
				t.Errorf("ValidateToken() adminID = %v, want %v", claims.AdminID, tt.expectAdmin)
			}
		})
	}
}

func TestValidateRefreshToken(t *testing.T) {
	cfg := &JWTConfig{
		SecretKey:     "test-secret-key-32-bytes-long!",
		AccessExpiry:  15 * time.Minute,
		RefreshExpiry: 7 * 24 * time.Hour,
	}
	auth := NewAuthService(cfg)

	// Generate a valid refresh token
	validToken, _ := auth.GenerateRefreshToken("admin-1", "company-1")

	tests := []struct {
		name        string
		token       string
		shouldErr   bool
		expectAdmin string
	}{
		{
			name:        "valid refresh token",
			token:       validToken,
			shouldErr:   false,
			expectAdmin: "admin-1",
		},
		{
			name:      "invalid refresh token",
			token:     "invalid.token.here",
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims, err := auth.ValidateRefreshToken(tt.token)
			if (err != nil) != tt.shouldErr {
				t.Errorf("ValidateRefreshToken() error = %v, shouldErr %v", err, tt.shouldErr)
				return
			}
			if err == nil && claims.AdminID != tt.expectAdmin {
				t.Errorf("ValidateRefreshToken() adminID = %v, want %v", claims.AdminID, tt.expectAdmin)
			}
		})
	}
}

func TestTokenExpiry(t *testing.T) {
	// Create auth service with very long expiry
	cfg := &JWTConfig{
		SecretKey:     "test-secret-key-32-bytes-long!",
		AccessExpiry:  24 * time.Hour,
		RefreshExpiry: 7 * 24 * time.Hour,
	}
	auth := NewAuthService(cfg)

	token, _ := auth.GenerateToken("admin-1", "company-1", "role-1")

	// Token should be valid for the duration
	claims, err := auth.ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken() failed on fresh token: %v", err)
	}

	// Verify expiry is set correctly
	if claims.ExpiresAt == nil {
		t.Error("Token should have ExpiresAt set")
	}
}

func TestRefreshAccessToken(t *testing.T) {
	cfg := &JWTConfig{
		SecretKey:     "test-secret-key-32-bytes-long!",
		AccessExpiry:  15 * time.Minute,
		RefreshExpiry: 7 * 24 * time.Hour,
	}
	auth := NewAuthService(cfg)

	// Generate refresh token
	refreshToken, _ := auth.GenerateRefreshToken("admin-1", "company-1")

	tests := []struct {
		name        string
		refreshToken string
		roleID      string
		shouldErr   bool
	}{
		{
			name:         "valid refresh",
			refreshToken: refreshToken,
			roleID:       "role-1",
			shouldErr:    false,
		},
		{
			name:         "invalid refresh token",
			refreshToken: "invalid.token",
			roleID:       "role-1",
			shouldErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newToken, err := auth.RefreshAccessToken(tt.refreshToken, tt.roleID)
			if (err != nil) != tt.shouldErr {
				t.Errorf("RefreshAccessToken() error = %v, shouldErr %v", err, tt.shouldErr)
				return
			}
			if err == nil && newToken == "" {
				t.Error("RefreshAccessToken() returned empty token")
			}
		})
	}
}

func TestHashPassword(t *testing.T) {
	password := "test-password-123"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	if hash == "" {
		t.Error("HashPassword() returned empty hash")
	}
	if hash == password {
		t.Error("HashPassword() returned unhashed password")
	}
}

func TestVerifyPassword(t *testing.T) {
	password := "test-password-123"
	hash, _ := HashPassword(password)

	tests := []struct {
		name      string
		password  string
		hash      string
		shouldErr bool
	}{
		{
			name:      "correct password",
			password:  password,
			hash:      hash,
			shouldErr: false,
		},
		{
			name:      "incorrect password",
			password:  "wrong-password",
			hash:      hash,
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := VerifyPassword(tt.password, tt.hash)
			if (err != nil) != tt.shouldErr {
				t.Errorf("VerifyPassword() error = %v, shouldErr %v", err, tt.shouldErr)
			}
		})
	}
}
