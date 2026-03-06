package auth

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository handles auth data persistence
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository creates a new auth repository
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// CreateUser creates a new user
func (r *Repository) CreateUser(ctx context.Context, user *User) error {
	user.ID = uuid.New().String()
	user.CreatedAt = time.Now()
	user.UpdatedAt = time.Now()
	user.Active = true

	_, err := r.db.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, first_name, last_name, role, active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, user.ID, user.Email, user.PasswordHash, user.FirstName, user.LastName, user.Role, user.Active, user.CreatedAt, user.UpdatedAt)

	return err
}

// GetUserByEmail finds a user by email
func (r *Repository) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	user := &User{}
	err := r.db.QueryRow(ctx, `
		SELECT id, email, password_hash, first_name, last_name, role, active, created_at, updated_at
		FROM users WHERE email = $1 AND active = true
	`, email).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.FirstName, &user.LastName, &user.Role, &user.Active, &user.CreatedAt, &user.UpdatedAt)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return user, err
}

// GetUserByID finds a user by ID
func (r *Repository) GetUserByID(ctx context.Context, id string) (*User, error) {
	user := &User{}
	err := r.db.QueryRow(ctx, `
		SELECT id, email, password_hash, first_name, last_name, role, active, created_at, updated_at
		FROM users WHERE id = $1 AND active = true
	`, id).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.FirstName, &user.LastName, &user.Role, &user.Active, &user.CreatedAt, &user.UpdatedAt)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return user, err
}

// SaveRefreshToken stores a refresh token
func (r *Repository) SaveRefreshToken(ctx context.Context, token *RefreshToken) error {
	token.ID = uuid.New().String()
	token.CreatedAt = time.Now()

	_, err := r.db.Exec(ctx, `
		INSERT INTO refresh_tokens (id, user_id, token, expires_at, created_at, revoked)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, token.ID, token.UserID, token.Token, token.ExpiresAt, token.CreatedAt, false)

	return err
}

// GetRefreshToken retrieves a refresh token
func (r *Repository) GetRefreshToken(ctx context.Context, token string) (*RefreshToken, error) {
	rt := &RefreshToken{}
	err := r.db.QueryRow(ctx, `
		SELECT id, user_id, token, expires_at, created_at, revoked
		FROM refresh_tokens WHERE token = $1 AND revoked = false AND expires_at > NOW()
	`, token).Scan(&rt.ID, &rt.UserID, &rt.Token, &rt.ExpiresAt, &rt.CreatedAt, &rt.Revoked)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return rt, err
}

// RevokeRefreshToken revokes a refresh token (for rotation)
func (r *Repository) RevokeRefreshToken(ctx context.Context, token string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE refresh_tokens SET revoked = true WHERE token = $1
	`, token)
	return err
}

// RevokeAllUserTokens revokes all refresh tokens for a user (logout from all devices)
func (r *Repository) RevokeAllUserTokens(ctx context.Context, userID string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE refresh_tokens SET revoked = true WHERE user_id = $1
	`, userID)
	return err
}

// CleanupExpiredTokens removes expired tokens (called periodically)
func (r *Repository) CleanupExpiredTokens(ctx context.Context) error {
	_, err := r.db.Exec(ctx, `
		DELETE FROM refresh_tokens WHERE expires_at < NOW() OR revoked = true
	`)
	return err
}
