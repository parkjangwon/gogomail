package admin

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidToken      = fmt.Errorf("invalid or expired token")
	ErrInvalidPassword   = fmt.Errorf("invalid password")
	ErrMissingToken      = fmt.Errorf("missing or malformed token")
	ErrInvalidClaims     = fmt.Errorf("invalid token claims")
)

// JWTConfig holds JWT configuration.
type JWTConfig struct {
	SecretKey     string
	AccessExpiry  time.Duration
	RefreshExpiry time.Duration
}

// TokenClaims holds the claims in a JWT token.
type TokenClaims struct {
	AdminID   string `json:"admin_id"`
	CompanyID string `json:"company_id"`
	RoleID    string `json:"role_id"`
	jwt.RegisteredClaims
}

// RefreshTokenClaims holds the claims for refresh tokens.
type RefreshTokenClaims struct {
	AdminID   string `json:"admin_id"`
	CompanyID string `json:"company_id"`
	jwt.RegisteredClaims
}

// AuthService handles JWT operations.
type AuthService struct {
	config *JWTConfig
}

// NewAuthService creates a new auth service.
func NewAuthService(config *JWTConfig) *AuthService {
	return &AuthService{config: config}
}

// GenerateToken generates an access token.
func (a *AuthService) GenerateToken(adminID, companyID, roleID string) (string, error) {
	if adminID == "" {
		return "", fmt.Errorf("%w: adminID", ErrMissingRequiredField)
	}
	if companyID == "" {
		return "", fmt.Errorf("%w: companyID", ErrMissingRequiredField)
	}

	now := time.Now()
	claims := TokenClaims{
		AdminID:   adminID,
		CompanyID: companyID,
		RoleID:    roleID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(a.config.AccessExpiry)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now.Add(-1 * time.Second)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(a.config.SecretKey))
}

// GenerateRefreshToken generates a refresh token.
func (a *AuthService) GenerateRefreshToken(adminID, companyID string) (string, error) {
	if adminID == "" {
		return "", fmt.Errorf("%w: adminID", ErrMissingRequiredField)
	}
	if companyID == "" {
		return "", fmt.Errorf("%w: companyID", ErrMissingRequiredField)
	}

	now := time.Now()
	claims := RefreshTokenClaims{
		AdminID:   adminID,
		CompanyID: companyID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(a.config.RefreshExpiry)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now.Add(-1 * time.Second)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(a.config.SecretKey))
}

// ValidateToken validates an access token and returns its claims.
func (a *AuthService) ValidateToken(tokenString string) (*TokenClaims, error) {
	if tokenString == "" {
		return nil, ErrMissingToken
	}

	claims := &TokenClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(a.config.SecretKey), nil
	})

	if err != nil {
		return nil, ErrInvalidToken
	}

	if !token.Valid {
		return nil, ErrInvalidToken
	}

	if claims.AdminID == "" {
		return nil, ErrInvalidClaims
	}

	return claims, nil
}

// ValidateRefreshToken validates a refresh token and returns its claims.
func (a *AuthService) ValidateRefreshToken(tokenString string) (*RefreshTokenClaims, error) {
	if tokenString == "" {
		return nil, ErrMissingToken
	}

	claims := &RefreshTokenClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(a.config.SecretKey), nil
	})

	if err != nil {
		return nil, ErrInvalidToken
	}

	if !token.Valid {
		return nil, ErrInvalidToken
	}

	if claims.AdminID == "" {
		return nil, ErrInvalidClaims
	}

	return claims, nil
}

// RefreshAccessToken generates a new access token from a refresh token.
func (a *AuthService) RefreshAccessToken(refreshToken, roleID string) (string, error) {
	claims, err := a.ValidateRefreshToken(refreshToken)
	if err != nil {
		return "", err
	}

	return a.GenerateToken(claims.AdminID, claims.CompanyID, roleID)
}

// HashPassword hashes a password using bcrypt.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// VerifyPassword verifies a password against a hash.
func VerifyPassword(password, hash string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}
