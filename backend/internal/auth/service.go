package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/argon2"

	"salusdomi.com/api/internal/common"
	"salusdomi.com/api/internal/config"
	"salusdomi.com/api/internal/domain"
)

// Argon2id parameters. These are deliberately conservative — memory is the
// primary cost driver against GPU attacks.
const (
	argon2Memory      uint32 = 64 * 1024 // 64 MB
	argon2Iterations  uint32 = 3
	argon2Parallelism uint8  = 2
	argon2SaltLength         = 16
	argon2KeyLength   uint32 = 32
)

// RegisterInput carries the fields required to create a new user account.
type RegisterInput struct {
	Email    string
	Password string
	Role     string // CLIENT | PRO — ADMIN is never created via API
}

// LoginInput carries the credentials for an authentication attempt.
type LoginInput struct {
	Email    string
	Password string
}

// AuthTokens holds both tokens produced during login/register/refresh.
// AccessToken is returned in the response body; RawRefreshToken is set as an httpOnly cookie.
type AuthTokens struct {
	AccessToken     string
	RawRefreshToken string
}

// Service defines the business logic for the auth domain.
type Service interface {
	Register(ctx context.Context, input RegisterInput) (*domain.User, *AuthTokens, error)
	Login(ctx context.Context, input LoginInput) (*domain.User, *AuthTokens, error)
	// RefreshToken rotates the refresh token and issues a new access token.
	RefreshToken(ctx context.Context, rawToken string) (*domain.User, *AuthTokens, error)
	Logout(ctx context.Context, rawToken string) error
}

type serviceImpl struct {
	repo Repository
	cfg  config.AuthConfig
}

// NewService returns a Service backed by the given Repository and config.
func NewService(repo Repository, cfg config.AuthConfig) Service {
	return &serviceImpl{repo: repo, cfg: cfg}
}

func (s *serviceImpl) Register(ctx context.Context, input RegisterInput) (*domain.User, *AuthTokens, error) {
	hash, err := hashPassword(input.Password)
	if err != nil {
		return nil, nil, common.Internal("HASH_ERROR", "failed to process password")
	}

	now := time.Now()
	user := &domain.User{
		ID:           uuid.New().String(),
		Email:        input.Email,
		PasswordHash: hash,
		Role:         input.Role,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.repo.SaveUser(ctx, user); err != nil {
		return nil, nil, err
	}

	tokens, err := s.issueTokens(ctx, user)
	if err != nil {
		return nil, nil, err
	}

	return user, tokens, nil
}

func (s *serviceImpl) Login(ctx context.Context, input LoginInput) (*domain.User, *AuthTokens, error) {
	user, err := s.repo.FindUserByEmail(ctx, input.Email)
	if err != nil {
		// Map any lookup failure to a generic message to prevent email enumeration.
		return nil, nil, common.Unauthorized("INVALID_CREDENTIALS", "invalid email or password")
	}

	match, err := verifyPassword(input.Password, user.PasswordHash)
	if err != nil || !match {
		return nil, nil, common.Unauthorized("INVALID_CREDENTIALS", "invalid email or password")
	}

	tokens, err := s.issueTokens(ctx, user)
	if err != nil {
		return nil, nil, err
	}

	return user, tokens, nil
}

func (s *serviceImpl) RefreshToken(ctx context.Context, rawToken string) (*domain.User, *AuthTokens, error) {
	tokenHash := hashRefreshToken(rawToken)

	userID, expiresAt, err := s.repo.FindRefreshToken(ctx, tokenHash)
	if err != nil {
		return nil, nil, err
	}

	if time.Now().After(expiresAt) {
		_ = s.repo.DeleteRefreshToken(ctx, tokenHash)
		return nil, nil, common.Unauthorized("REFRESH_TOKEN_EXPIRED", "refresh token has expired")
	}

	// Rotate: delete the old token before issuing a new one.
	if err := s.repo.DeleteRefreshToken(ctx, tokenHash); err != nil {
		return nil, nil, err
	}

	user, err := s.repo.FindUserByID(ctx, userID)
	if err != nil {
		return nil, nil, err
	}

	tokens, err := s.issueTokens(ctx, user)
	if err != nil {
		return nil, nil, err
	}

	return user, tokens, nil
}

func (s *serviceImpl) Logout(ctx context.Context, rawToken string) error {
	return s.repo.DeleteRefreshToken(ctx, hashRefreshToken(rawToken))
}

// issueTokens mints a JWT access token and a new refresh token, persisting the refresh token hash.
func (s *serviceImpl) issueTokens(ctx context.Context, user *domain.User) (*AuthTokens, error) {
	accessToken, err := issueJWT(user.ID, user.Role, s.cfg.JWTSecret, s.cfg.JWTExpiryMinutes)
	if err != nil {
		return nil, common.Internal("TOKEN_ERROR", "failed to issue access token")
	}

	rawRefreshToken, tokenHash, err := generateRefreshToken()
	if err != nil {
		return nil, common.Internal("TOKEN_ERROR", "failed to generate refresh token")
	}

	expiresAt := time.Now().Add(time.Duration(s.cfg.RefreshTokenExpiryDays) * 24 * time.Hour)
	if err := s.repo.SaveRefreshToken(ctx, user.ID, tokenHash, expiresAt); err != nil {
		return nil, err
	}

	return &AuthTokens{
		AccessToken:     accessToken,
		RawRefreshToken: rawRefreshToken,
	}, nil
}

// ── JWT helpers ──────────────────────────────────────────────────────────────

type jwtClaims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

func issueJWT(userID, role, secret string, expiryMinutes int) (string, error) {
	now := time.Now()
	claims := jwtClaims{
		Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Duration(expiryMinutes) * time.Minute)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}

// validateJWT parses and validates a signed JWT string, returning the claims.
// Exported at package level so middleware can use it without a service instance.
func validateJWT(tokenStr, secret string) (*jwtClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &jwtClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*jwtClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token claims")
	}
	return claims, nil
}

// ── Argon2id helpers ─────────────────────────────────────────────────────────

// hashPassword returns a PHC-formatted Argon2id hash string.
func hashPassword(password string) (string, error) {
	salt := make([]byte, argon2SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(password), salt, argon2Iterations, argon2Memory, argon2Parallelism, argon2KeyLength)
	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		argon2Memory, argon2Iterations, argon2Parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	), nil
}

// verifyPassword checks a plaintext password against a PHC Argon2id hash.
func verifyPassword(password, encodedHash string) (bool, error) {
	memory, iterations, parallelism, salt, storedHash, err := parseArgon2Hash(encodedHash)
	if err != nil {
		return false, err
	}
	keyLen := uint32(len(storedHash))
	computed := argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, keyLen)
	return subtle.ConstantTimeCompare(storedHash, computed) == 1, nil
}

func parseArgon2Hash(encoded string) (memory uint32, iterations uint32, parallelism uint8, salt, hash []byte, err error) {
	// Expected format: $argon2id$v=19$m=65536,t=3,p=2$<b64salt>$<b64hash>
	var version int
	parts := splitN(encoded, "$", 6)
	if len(parts) != 6 || parts[1] != "argon2id" {
		return 0, 0, 0, nil, nil, errors.New("invalid argon2id hash format")
	}
	if _, err = fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return 0, 0, 0, nil, nil, fmt.Errorf("invalid argon2id version: %w", err)
	}
	if _, err = fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism); err != nil {
		return 0, 0, 0, nil, nil, fmt.Errorf("invalid argon2id params: %w", err)
	}
	if salt, err = base64.RawStdEncoding.DecodeString(parts[4]); err != nil {
		return 0, 0, 0, nil, nil, fmt.Errorf("invalid argon2id salt: %w", err)
	}
	if hash, err = base64.RawStdEncoding.DecodeString(parts[5]); err != nil {
		return 0, 0, 0, nil, nil, fmt.Errorf("invalid argon2id hash: %w", err)
	}
	return memory, iterations, parallelism, salt, hash, nil
}

// splitN splits s by sep into exactly n parts, padding with empty strings if needed.
// Avoids using strings.Split which would include the leading empty string for "$argon2id...".
func splitN(s, sep string, n int) []string {
	parts := make([]string, 0, n)
	for i := 0; i < n-1; i++ {
		idx := indexOf(s, sep)
		if idx < 0 {
			break
		}
		parts = append(parts, s[:idx])
		s = s[idx+len(sep):]
	}
	parts = append(parts, s)
	return parts
}

func indexOf(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// ── Refresh token helpers ────────────────────────────────────────────────────

// generateRefreshToken returns a 32-byte cryptographically random token
// (hex-encoded) and its SHA-256 hash for storage.
func generateRefreshToken() (rawToken, tokenHash string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", err
	}
	rawToken = hex.EncodeToString(b)
	return rawToken, hashRefreshToken(rawToken), nil
}

// hashRefreshToken returns the hex-encoded SHA-256 hash of a raw refresh token.
func hashRefreshToken(rawToken string) string {
	sum := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(sum[:])
}
