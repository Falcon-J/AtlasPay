package auth

import (
	"context"
	"time"

	commonauth "github.com/atlaspay/platform/internal/common/auth"
	"github.com/atlaspay/platform/internal/common/errors"
)

// Service handles auth business logic
type Service struct {
	repo       *Repository
	jwtManager *commonauth.JWTManager
	refreshExp time.Duration
}

// NewService creates a new auth service
func NewService(repo *Repository, jwtManager *commonauth.JWTManager, refreshExp time.Duration) *Service {
	return &Service{
		repo:       repo,
		jwtManager: jwtManager,
		refreshExp: refreshExp,
	}
}

// Register creates a new user account
func (s *Service) Register(ctx context.Context, req *RegisterRequest) (*AuthResponse, error) {
	// Check if user exists
	existing, err := s.repo.GetUserByEmail(ctx, req.Email)
	if err != nil {
		return nil, errors.ErrInternalServer.WithDetails(err.Error())
	}
	if existing != nil {
		return nil, errors.ErrUserExists
	}

	// Hash password
	hash, err := HashPassword(req.Password)
	if err != nil {
		return nil, errors.ErrInternalServer.WithDetails("failed to hash password")
	}

	// Create user
	user := &User{
		Email:        req.Email,
		PasswordHash: hash,
		FirstName:    req.FirstName,
		LastName:     req.LastName,
		Role:         string(commonauth.RoleUser), // Default role
	}

	if err := s.repo.CreateUser(ctx, user); err != nil {
		return nil, errors.ErrInternalServer.WithDetails(err.Error())
	}

	// Generate tokens
	return s.generateAuthResponse(ctx, user)
}

// Login authenticates a user and returns tokens
func (s *Service) Login(ctx context.Context, req *LoginRequest) (*AuthResponse, error) {
	// Find user
	user, err := s.repo.GetUserByEmail(ctx, req.Email)
	if err != nil {
		return nil, errors.ErrInternalServer.WithDetails(err.Error())
	}
	if user == nil {
		return nil, errors.ErrInvalidCredentials
	}

	// Verify password
	if !CheckPassword(req.Password, user.PasswordHash) {
		return nil, errors.ErrInvalidCredentials
	}

	// Generate tokens
	return s.generateAuthResponse(ctx, user)
}

// RefreshTokens generates new token pair and rotates refresh token
func (s *Service) RefreshTokens(ctx context.Context, req *RefreshRequest) (*AuthResponse, error) {
	// Validate refresh token
	claims, err := s.jwtManager.ValidateRefreshToken(req.RefreshToken)
	if err != nil {
		return nil, errors.ErrInvalidToken.WithDetails(err.Error())
	}

	// Check if token exists in DB (not revoked)
	storedToken, err := s.repo.GetRefreshToken(ctx, req.RefreshToken)
	if err != nil {
		return nil, errors.ErrInternalServer.WithDetails(err.Error())
	}
	if storedToken == nil {
		return nil, errors.ErrInvalidToken.WithDetails("token has been revoked")
	}

	// Get user
	user, err := s.repo.GetUserByID(ctx, claims.UserID)
	if err != nil {
		return nil, errors.ErrInternalServer.WithDetails(err.Error())
	}
	if user == nil {
		return nil, errors.ErrNotFound.WithDetails("user not found")
	}

	// Revoke old token (token rotation for security)
	if err := s.repo.RevokeRefreshToken(ctx, req.RefreshToken); err != nil {
		return nil, errors.ErrInternalServer.WithDetails(err.Error())
	}

	// Generate new tokens
	return s.generateAuthResponse(ctx, user)
}

// Logout revokes the refresh token
func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	return s.repo.RevokeRefreshToken(ctx, refreshToken)
}

// LogoutAll revokes all tokens for a user
func (s *Service) LogoutAll(ctx context.Context, userID string) error {
	return s.repo.RevokeAllUserTokens(ctx, userID)
}

func (s *Service) generateAuthResponse(ctx context.Context, user *User) (*AuthResponse, error) {
	// Generate token pair
	tokens, err := s.jwtManager.GenerateTokenPair(user.ID, user.Email, commonauth.Role(user.Role))
	if err != nil {
		return nil, errors.ErrInternalServer.WithDetails("failed to generate tokens")
	}

	// Store refresh token
	rt := &RefreshToken{
		UserID:    user.ID,
		Token:     tokens.RefreshToken,
		ExpiresAt: time.Now().Add(s.refreshExp),
	}
	if err := s.repo.SaveRefreshToken(ctx, rt); err != nil {
		return nil, errors.ErrInternalServer.WithDetails(err.Error())
	}

	return &AuthResponse{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		ExpiresIn:    tokens.ExpiresIn,
		User:         user,
	}, nil
}

// GetUserByID retrieves a user by ID
func (s *Service) GetUserByID(ctx context.Context, id string) (*User, error) {
	return s.repo.GetUserByID(ctx, id)
}
