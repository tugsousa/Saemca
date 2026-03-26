package auth_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"salusdomi.com/api/internal/auth"
	"salusdomi.com/api/internal/domain"
)

// setupDB starts a PostgreSQL+PostGIS container and returns a connection pool
// plus a cleanup function. The schema is created inline — no migration runner needed.
func setupDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx,
		"postgis/postgis:16-3.4",
		postgres.WithDatabase("salusdomi_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2),
		),
	)
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = pgContainer.Terminate(ctx)
	})

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	applySchema(t, pool)
	return pool
}

// applySchema creates only the tables needed for the auth domain tests.
func applySchema(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `
		CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

		CREATE TABLE IF NOT EXISTS users (
			id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			email            TEXT UNIQUE NOT NULL,
			password_hash    TEXT NOT NULL,
			role             TEXT CHECK (role IN ('CLIENT', 'PRO', 'ADMIN')),
			stripe_customer_id TEXT,
			created_at       TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at       TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		);

		CREATE TABLE IF NOT EXISTS refresh_tokens (
			id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			token_hash TEXT NOT NULL UNIQUE,
			expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		);

		CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user_id    ON refresh_tokens(user_id);
		CREATE INDEX IF NOT EXISTS idx_refresh_tokens_expires_at ON refresh_tokens(expires_at);
	`)
	require.NoError(t, err)
}

// truncate clears all test data between tests.
func truncate(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `TRUNCATE refresh_tokens, users CASCADE`)
	require.NoError(t, err)
}

// seedUser inserts a test user directly and returns the domain model.
func seedUser(t *testing.T, repo auth.Repository, email, role string) *domain.User {
	t.Helper()
	user := &domain.User{
		ID:           fmt.Sprintf("00000000-0000-0000-0000-%012d", 1),
		Email:        email,
		PasswordHash: "$argon2id$v=19$m=65536,t=3,p=2$dGVzdHNhbHQ$dGVzdGhhc2g",
		Role:         role,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	// Use a proper UUID so it doesn't violate format constraints.
	user.ID = "11111111-1111-1111-1111-111111111111"
	err := repo.SaveUser(context.Background(), user)
	require.NoError(t, err)
	return user
}

// ── SaveUser ──────────────────────────────────────────────────────────────────

func TestRepository_SaveUser_HappyPath(t *testing.T) {
	pool := setupDB(t)
	defer truncate(t, pool)
	repo := auth.NewRepository(pool)

	user := &domain.User{
		ID:           "22222222-2222-2222-2222-222222222222",
		Email:        "alice@example.com",
		PasswordHash: "hash",
		Role:         "CLIENT",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	err := repo.SaveUser(context.Background(), user)
	require.NoError(t, err)
}

func TestRepository_SaveUser_DuplicateEmail(t *testing.T) {
	pool := setupDB(t)
	defer truncate(t, pool)
	repo := auth.NewRepository(pool)

	user := &domain.User{
		ID: "33333333-3333-3333-3333-333333333333", Email: "dup@example.com",
		PasswordHash: "hash", Role: "CLIENT", CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	require.NoError(t, repo.SaveUser(context.Background(), user))

	user2 := &domain.User{
		ID: "44444444-4444-4444-4444-444444444444", Email: "dup@example.com",
		PasswordHash: "hash", Role: "PRO", CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	err := repo.SaveUser(context.Background(), user2)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

// ── FindUserByEmail ───────────────────────────────────────────────────────────

func TestRepository_FindUserByEmail_Found(t *testing.T) {
	pool := setupDB(t)
	defer truncate(t, pool)
	repo := auth.NewRepository(pool)

	seeded := seedUser(t, repo, "bob@example.com", "PRO")

	found, err := repo.FindUserByEmail(context.Background(), "bob@example.com")
	require.NoError(t, err)
	assert.Equal(t, seeded.ID, found.ID)
	assert.Equal(t, "bob@example.com", found.Email)
	assert.Equal(t, "PRO", found.Role)
}

func TestRepository_FindUserByEmail_NotFound(t *testing.T) {
	pool := setupDB(t)
	defer truncate(t, pool)
	repo := auth.NewRepository(pool)

	_, err := repo.FindUserByEmail(context.Background(), "nobody@example.com")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// ── FindUserByID ──────────────────────────────────────────────────────────────

func TestRepository_FindUserByID_Found(t *testing.T) {
	pool := setupDB(t)
	defer truncate(t, pool)
	repo := auth.NewRepository(pool)

	seeded := seedUser(t, repo, "carol@example.com", "CLIENT")

	found, err := repo.FindUserByID(context.Background(), seeded.ID)
	require.NoError(t, err)
	assert.Equal(t, seeded.ID, found.ID)
}

func TestRepository_FindUserByID_NotFound(t *testing.T) {
	pool := setupDB(t)
	defer truncate(t, pool)
	repo := auth.NewRepository(pool)

	_, err := repo.FindUserByID(context.Background(), "00000000-0000-0000-0000-000000000000")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// ── Refresh token lifecycle ───────────────────────────────────────────────────

func TestRepository_RefreshToken_SaveFindDelete(t *testing.T) {
	pool := setupDB(t)
	defer truncate(t, pool)
	repo := auth.NewRepository(pool)

	user := seedUser(t, repo, "dave@example.com", "CLIENT")
	expiresAt := time.Now().Add(30 * 24 * time.Hour).UTC().Truncate(time.Millisecond)

	err := repo.SaveRefreshToken(context.Background(), user.ID, "testhash123", expiresAt)
	require.NoError(t, err)

	userID, gotExpiry, err := repo.FindRefreshToken(context.Background(), "testhash123")
	require.NoError(t, err)
	assert.Equal(t, user.ID, userID)
	assert.WithinDuration(t, expiresAt, gotExpiry, time.Second)

	err = repo.DeleteRefreshToken(context.Background(), "testhash123")
	require.NoError(t, err)

	_, _, err = repo.FindRefreshToken(context.Background(), "testhash123")
	require.Error(t, err)
}

func TestRepository_DeleteAllUserTokens(t *testing.T) {
	pool := setupDB(t)
	defer truncate(t, pool)
	repo := auth.NewRepository(pool)

	user := seedUser(t, repo, "eve@example.com", "PRO")
	exp := time.Now().Add(time.Hour)

	require.NoError(t, repo.SaveRefreshToken(context.Background(), user.ID, "hash-a", exp))
	require.NoError(t, repo.SaveRefreshToken(context.Background(), user.ID, "hash-b", exp))

	err := repo.DeleteAllUserTokens(context.Background(), user.ID)
	require.NoError(t, err)

	_, _, err = repo.FindRefreshToken(context.Background(), "hash-a")
	require.Error(t, err)
	_, _, err = repo.FindRefreshToken(context.Background(), "hash-b")
	require.Error(t, err)
}
