package auth_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"salusdomi.com/api/internal/auth"
	"salusdomi.com/api/internal/common"
	"salusdomi.com/api/internal/config"
	"salusdomi.com/api/internal/domain"
)

// ── Mock repository ───────────────────────────────────────────────────────────

type mockRepo struct {
	users         map[string]*domain.User
	usersByID     map[string]*domain.User
	refreshTokens map[string]mockToken
}

type mockToken struct {
	userID    string
	expiresAt time.Time
}

func newMockRepo() *mockRepo {
	return &mockRepo{
		users:         make(map[string]*domain.User),
		usersByID:     make(map[string]*domain.User),
		refreshTokens: make(map[string]mockToken),
	}
}

func (m *mockRepo) SaveUser(_ context.Context, user *domain.User) error {
	if _, exists := m.users[user.Email]; exists {
		return common.Conflict("EMAIL_ALREADY_EXISTS", "an account with this email already exists")
	}
	m.users[user.Email] = user
	m.usersByID[user.ID] = user
	return nil
}

func (m *mockRepo) FindUserByEmail(_ context.Context, email string) (*domain.User, error) {
	user, ok := m.users[email]
	if !ok {
		return nil, common.NotFound("USER_NOT_FOUND", "user not found")
	}
	return user, nil
}

func (m *mockRepo) FindUserByID(_ context.Context, id string) (*domain.User, error) {
	user, ok := m.usersByID[id]
	if !ok {
		return nil, common.NotFound("USER_NOT_FOUND", "user not found")
	}
	return user, nil
}

func (m *mockRepo) SaveRefreshToken(_ context.Context, userID, tokenHash string, expiresAt time.Time) error {
	m.refreshTokens[tokenHash] = mockToken{userID: userID, expiresAt: expiresAt}
	return nil
}

func (m *mockRepo) FindRefreshToken(_ context.Context, tokenHash string) (string, time.Time, error) {
	t, ok := m.refreshTokens[tokenHash]
	if !ok {
		return "", time.Time{}, common.Unauthorized("INVALID_REFRESH_TOKEN", "refresh token not found")
	}
	return t.userID, t.expiresAt, nil
}

func (m *mockRepo) DeleteRefreshToken(_ context.Context, tokenHash string) error {
	delete(m.refreshTokens, tokenHash)
	return nil
}

func (m *mockRepo) DeleteAllUserTokens(_ context.Context, userID string) error {
	for k, v := range m.refreshTokens {
		if v.userID == userID {
			delete(m.refreshTokens, k)
		}
	}
	return nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func testCfg() config.AuthConfig {
	return config.AuthConfig{
		JWTSecret:              "test-secret-must-be-long-enough-256bit",
		JWTExpiryMinutes:       15,
		RefreshTokenExpiryDays: 30,
		CookieDomain:           "localhost",
		CookieSecure:           false,
	}
}

func newService() (auth.Service, *mockRepo) {
	repo := newMockRepo()
	svc := auth.NewService(repo, testCfg())
	return svc, repo
}

// ── Register tests ────────────────────────────────────────────────────────────

func TestRegister_HappyPath(t *testing.T) {
	svc, repo := newService()

	user, tokens, err := svc.Register(context.Background(), auth.RegisterInput{
		Email:    "alice@example.com",
		Password: "securepass",
		Role:     "CLIENT",
	})

	require.NoError(t, err)
	assert.Equal(t, "alice@example.com", user.Email)
	assert.Equal(t, "CLIENT", user.Role)
	assert.NotEmpty(t, user.ID)
	assert.NotEmpty(t, tokens.AccessToken)
	assert.NotEmpty(t, tokens.RawRefreshToken)

	// Refresh token must be persisted (as hash, not raw).
	assert.NotContains(t, repo.refreshTokens, tokens.RawRefreshToken,
		"raw token must not be stored — only its hash")
	assert.Len(t, repo.refreshTokens, 1)
}

func TestRegister_DuplicateEmail(t *testing.T) {
	svc, _ := newService()
	input := auth.RegisterInput{Email: "dup@example.com", Password: "securepass", Role: "CLIENT"}

	_, _, err := svc.Register(context.Background(), input)
	require.NoError(t, err)

	_, _, err = svc.Register(context.Background(), input)
	require.Error(t, err)

	appErr, ok := err.(*common.AppError)
	require.True(t, ok, "expected AppError")
	assert.Equal(t, "EMAIL_ALREADY_EXISTS", appErr.Code)
}

// ── Login tests ───────────────────────────────────────────────────────────────

func TestLogin_HappyPath(t *testing.T) {
	svc, _ := newService()

	_, _, err := svc.Register(context.Background(), auth.RegisterInput{
		Email: "bob@example.com", Password: "mypassword", Role: "PRO",
	})
	require.NoError(t, err)

	user, tokens, err := svc.Login(context.Background(), auth.LoginInput{
		Email: "bob@example.com", Password: "mypassword",
	})

	require.NoError(t, err)
	assert.Equal(t, "bob@example.com", user.Email)
	assert.NotEmpty(t, tokens.AccessToken)
	assert.NotEmpty(t, tokens.RawRefreshToken)
}

func TestLogin_WrongPassword(t *testing.T) {
	svc, _ := newService()

	_, _, err := svc.Register(context.Background(), auth.RegisterInput{
		Email: "carol@example.com", Password: "correctpass", Role: "CLIENT",
	})
	require.NoError(t, err)

	_, _, err = svc.Login(context.Background(), auth.LoginInput{
		Email: "carol@example.com", Password: "wrongpass",
	})

	require.Error(t, err)
	appErr, ok := err.(*common.AppError)
	require.True(t, ok)
	assert.Equal(t, "INVALID_CREDENTIALS", appErr.Code)
}

func TestLogin_UserNotFound(t *testing.T) {
	svc, _ := newService()

	_, _, err := svc.Login(context.Background(), auth.LoginInput{
		Email: "nobody@example.com", Password: "anypass",
	})

	require.Error(t, err)
	appErr, ok := err.(*common.AppError)
	require.True(t, ok)
	// Must use the same generic code as wrong password — no email enumeration.
	assert.Equal(t, "INVALID_CREDENTIALS", appErr.Code)
}

// ── RefreshToken tests ────────────────────────────────────────────────────────

func TestRefreshToken_HappyPath(t *testing.T) {
	svc, repo := newService()

	_, tokens, err := svc.Register(context.Background(), auth.RegisterInput{
		Email: "dave@example.com", Password: "pass1234", Role: "CLIENT",
	})
	require.NoError(t, err)

	oldRaw := tokens.RawRefreshToken
	initialTokenCount := len(repo.refreshTokens)

	user2, tokens2, err := svc.RefreshToken(context.Background(), oldRaw)

	require.NoError(t, err)
	assert.Equal(t, "dave@example.com", user2.Email)
	assert.NotEmpty(t, tokens2.AccessToken)
	assert.NotEmpty(t, tokens2.RawRefreshToken)
	assert.NotEqual(t, oldRaw, tokens2.RawRefreshToken, "refresh token must rotate")
	// Old token deleted, new token saved — count stays the same.
	assert.Equal(t, initialTokenCount, len(repo.refreshTokens))
}

func TestRefreshToken_InvalidToken(t *testing.T) {
	svc, _ := newService()

	_, _, err := svc.RefreshToken(context.Background(), "not-a-valid-token")

	require.Error(t, err)
	appErr, ok := err.(*common.AppError)
	require.True(t, ok)
	assert.Equal(t, "INVALID_REFRESH_TOKEN", appErr.Code)
}

func TestRefreshToken_ExpiredToken(t *testing.T) {
	svc, repo := newService()

	_, tokens, err := svc.Register(context.Background(), auth.RegisterInput{
		Email: "eve@example.com", Password: "pass1234", Role: "CLIENT",
	})
	require.NoError(t, err)

	// Manually expire the token in the mock store.
	for k, v := range repo.refreshTokens {
		v.expiresAt = time.Now().Add(-1 * time.Hour)
		repo.refreshTokens[k] = v
	}

	_, _, err = svc.RefreshToken(context.Background(), tokens.RawRefreshToken)

	require.Error(t, err)
	appErr, ok := err.(*common.AppError)
	require.True(t, ok)
	assert.Equal(t, "REFRESH_TOKEN_EXPIRED", appErr.Code)
}

// ── Logout tests ──────────────────────────────────────────────────────────────

func TestLogout_HappyPath(t *testing.T) {
	svc, repo := newService()

	_, tokens, err := svc.Register(context.Background(), auth.RegisterInput{
		Email: "frank@example.com", Password: "pass1234", Role: "PRO",
	})
	require.NoError(t, err)
	require.Len(t, repo.refreshTokens, 1)

	err = svc.Logout(context.Background(), tokens.RawRefreshToken)

	require.NoError(t, err)
	assert.Empty(t, repo.refreshTokens, "refresh token must be deleted on logout")
}

func TestLogout_UnknownToken(t *testing.T) {
	svc, _ := newService()
	// Logging out with an unknown token should not error —
	// the desired state (token gone) is already satisfied.
	err := svc.Logout(context.Background(), "unknown-token")
	assert.NoError(t, err)
}
