package auth

import (
	"context"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Role represents user roles for RBAC
type Role string

const (
	RoleAdmin Role = "admin"
	RoleUser  Role = "user"
)

// Claims represents JWT claims
type Claims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Role   Role   `json:"role"`
	jwt.RegisteredClaims
}

// TokenPair represents access and refresh tokens
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

// JWTManager handles JWT operations
type JWTManager struct {
	accessSecret  []byte
	refreshSecret []byte
	accessExpiry  time.Duration
	refreshExpiry time.Duration
}

// NewJWTManager creates a new JWT manager
func NewJWTManager(accessSecret, refreshSecret string, accessExpiry, refreshExpiry time.Duration) *JWTManager {
	return &JWTManager{
		accessSecret:  []byte(accessSecret),
		refreshSecret: []byte(refreshSecret),
		accessExpiry:  accessExpiry,
		refreshExpiry: refreshExpiry,
	}
}

// GenerateTokenPair generates both access and refresh tokens
func (j *JWTManager) GenerateTokenPair(userID, email string, role Role) (*TokenPair, error) {
	// Access token
	accessClaims := &Claims{
		UserID: userID,
		Email:  email,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(j.accessExpiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   userID,
		},
	}

	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessTokenString, err := accessToken.SignedString(j.accessSecret)
	if err != nil {
		return nil, err
	}

	// Refresh token
	refreshClaims := &Claims{
		UserID: userID,
		Email:  email,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(j.refreshExpiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   userID,
		},
	}

	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshTokenString, err := refreshToken.SignedString(j.refreshSecret)
	if err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:  accessTokenString,
		RefreshToken: refreshTokenString,
		ExpiresIn:    int64(j.accessExpiry.Seconds()),
	}, nil
}

// ValidateAccessToken validates an access token and returns claims
func (j *JWTManager) ValidateAccessToken(tokenString string) (*Claims, error) {
	return j.validateToken(tokenString, j.accessSecret)
}

// ValidateRefreshToken validates a refresh token and returns claims
func (j *JWTManager) ValidateRefreshToken(tokenString string) (*Claims, error) {
	return j.validateToken(tokenString, j.refreshSecret)
}

func (j *JWTManager) validateToken(tokenString string, secret []byte) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return secret, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}

// Context key for storing user info
type contextKey string

const UserContextKey contextKey = "user"

// UserFromContext retrieves user claims from context
func UserFromContext(ctx context.Context) (*Claims, bool) {
	claims, ok := ctx.Value(UserContextKey).(*Claims)
	return claims, ok
}

// ContextWithUser stores user claims in context
func ContextWithUser(ctx context.Context, claims *Claims) context.Context {
	return context.WithValue(ctx, UserContextKey, claims)
}
