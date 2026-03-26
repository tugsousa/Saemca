package auth

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"salusdomi.com/api/internal/common"
	"salusdomi.com/api/internal/domain"
)

// Repository defines the data access operations for the auth domain.
type Repository interface {
	SaveUser(ctx context.Context, user *domain.User) error
	FindUserByEmail(ctx context.Context, email string) (*domain.User, error)
	FindUserByID(ctx context.Context, id string) (*domain.User, error)
	SaveRefreshToken(ctx context.Context, userID, tokenHash string, expiresAt time.Time) error
	FindRefreshToken(ctx context.Context, tokenHash string) (userID string, expiresAt time.Time, err error)
	DeleteRefreshToken(ctx context.Context, tokenHash string) error
	DeleteAllUserTokens(ctx context.Context, userID string) error
}

type pgRepository struct {
	db *pgxpool.Pool
}

// NewRepository returns a PostgreSQL-backed Repository.
func NewRepository(db *pgxpool.Pool) Repository {
	return &pgRepository{db: db}
}

func (r *pgRepository) SaveUser(ctx context.Context, user *domain.User) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, role, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, user.ID, user.Email, user.PasswordHash, user.Role, user.CreatedAt, user.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return common.Conflict("EMAIL_ALREADY_EXISTS", "an account with this email already exists")
		}
		return common.Internal("DB_ERROR", "failed to save user")
	}
	return nil
}

func (r *pgRepository) FindUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	user := &domain.User{}
	err := r.db.QueryRow(ctx, `
		SELECT id, email, password_hash, role, COALESCE(stripe_customer_id, ''), created_at, updated_at
		FROM users WHERE email = $1
	`, email).Scan(
		&user.ID, &user.Email, &user.PasswordHash, &user.Role,
		&user.StripeCustomerID, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, common.NotFound("USER_NOT_FOUND", "user not found")
		}
		return nil, common.Internal("DB_ERROR", "failed to query user")
	}
	return user, nil
}

func (r *pgRepository) FindUserByID(ctx context.Context, id string) (*domain.User, error) {
	user := &domain.User{}
	err := r.db.QueryRow(ctx, `
		SELECT id, email, password_hash, role, COALESCE(stripe_customer_id, ''), created_at, updated_at
		FROM users WHERE id = $1
	`, id).Scan(
		&user.ID, &user.Email, &user.PasswordHash, &user.Role,
		&user.StripeCustomerID, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, common.NotFound("USER_NOT_FOUND", "user not found")
		}
		return nil, common.Internal("DB_ERROR", "failed to query user")
	}
	return user, nil
}

func (r *pgRepository) SaveRefreshToken(ctx context.Context, userID, tokenHash string, expiresAt time.Time) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
	`, userID, tokenHash, expiresAt)
	if err != nil {
		return common.Internal("DB_ERROR", "failed to save refresh token")
	}
	return nil
}

func (r *pgRepository) FindRefreshToken(ctx context.Context, tokenHash string) (string, time.Time, error) {
	var userID string
	var expiresAt time.Time
	err := r.db.QueryRow(ctx, `
		SELECT user_id, expires_at FROM refresh_tokens WHERE token_hash = $1
	`, tokenHash).Scan(&userID, &expiresAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", time.Time{}, common.Unauthorized("INVALID_REFRESH_TOKEN", "refresh token not found")
		}
		return "", time.Time{}, common.Internal("DB_ERROR", "failed to query refresh token")
	}
	return userID, expiresAt, nil
}

func (r *pgRepository) DeleteRefreshToken(ctx context.Context, tokenHash string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM refresh_tokens WHERE token_hash = $1`, tokenHash)
	if err != nil {
		return common.Internal("DB_ERROR", "failed to delete refresh token")
	}
	return nil
}

func (r *pgRepository) DeleteAllUserTokens(ctx context.Context, userID string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM refresh_tokens WHERE user_id = $1`, userID)
	if err != nil {
		return common.Internal("DB_ERROR", "failed to delete user tokens")
	}
	return nil
}

// isUniqueViolation returns true when err is a PostgreSQL unique constraint violation (23505).
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
