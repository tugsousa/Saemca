package auth

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"salusdomi.com/api/internal/common"
	"salusdomi.com/api/internal/config"
)

const refreshTokenCookie = "refresh_token"

// Handler handles HTTP requests for the auth domain.
type Handler struct {
	svc Service
	cfg config.AuthConfig
}

// NewHandler wires the service and config into an HTTP handler.
func NewHandler(svc Service, cfg config.AuthConfig) *Handler {
	return &Handler{svc: svc, cfg: cfg}
}

// Routes returns a sub-router with all auth endpoints mounted.
func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Post("/register", h.register)
	r.Post("/login", h.login)
	r.Post("/refresh", h.refresh)
	r.Post("/logout", h.logout)
	return r
}

// ── Request / Response types ─────────────────────────────────────────────────

type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type userDTO struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

type authResponse struct {
	AccessToken string  `json:"access_token"`
	User        userDTO `json:"user"`
}

// ── Handlers ─────────────────────────────────────────────────────────────────

// POST /auth/register
func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.WriteError(w, r, http.StatusBadRequest, "INVALID_JSON", "request body must be valid JSON")
		return
	}

	if err := validateRegisterRequest(req); err != nil {
		common.WriteAppError(w, r, err)
		return
	}

	user, tokens, err := h.svc.Register(r.Context(), RegisterInput{
		Email:    strings.ToLower(strings.TrimSpace(req.Email)),
		Password: req.Password,
		Role:     req.Role,
	})
	if err != nil {
		if appErr, ok := err.(*common.AppError); ok {
			common.WriteAppError(w, r, appErr)
			return
		}
		slog.ErrorContext(r.Context(), "register failed", "error", err)
		common.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "registration failed")
		return
	}

	h.setRefreshCookie(w, tokens.RawRefreshToken)
	common.WriteSuccess(w, r, http.StatusCreated, authResponse{
		AccessToken: tokens.AccessToken,
		User:        userDTO{ID: user.ID, Email: user.Email, Role: user.Role},
	})
}

// POST /auth/login
func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.WriteError(w, r, http.StatusBadRequest, "INVALID_JSON", "request body must be valid JSON")
		return
	}

	if strings.TrimSpace(req.Email) == "" || req.Password == "" {
		common.WriteError(w, r, http.StatusBadRequest, "MISSING_FIELDS", "email and password are required")
		return
	}

	user, tokens, err := h.svc.Login(r.Context(), LoginInput{
		Email:    strings.ToLower(strings.TrimSpace(req.Email)),
		Password: req.Password,
	})
	if err != nil {
		if appErr, ok := err.(*common.AppError); ok {
			common.WriteAppError(w, r, appErr)
			return
		}
		slog.ErrorContext(r.Context(), "login failed", "error", err)
		common.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "login failed")
		return
	}

	h.setRefreshCookie(w, tokens.RawRefreshToken)
	common.WriteSuccess(w, r, http.StatusOK, authResponse{
		AccessToken: tokens.AccessToken,
		User:        userDTO{ID: user.ID, Email: user.Email, Role: user.Role},
	})
}

// POST /auth/refresh
func (h *Handler) refresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(refreshTokenCookie)
	if err != nil {
		common.WriteError(w, r, http.StatusUnauthorized, "MISSING_REFRESH_TOKEN", "refresh token cookie is required")
		return
	}

	user, tokens, err := h.svc.RefreshToken(r.Context(), cookie.Value)
	if err != nil {
		if appErr, ok := err.(*common.AppError); ok {
			common.WriteAppError(w, r, appErr)
			return
		}
		slog.ErrorContext(r.Context(), "token refresh failed", "error", err)
		common.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "token refresh failed")
		return
	}

	h.setRefreshCookie(w, tokens.RawRefreshToken)
	common.WriteSuccess(w, r, http.StatusOK, authResponse{
		AccessToken: tokens.AccessToken,
		User:        userDTO{ID: user.ID, Email: user.Email, Role: user.Role},
	})
}

// POST /auth/logout
func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(refreshTokenCookie)
	if err != nil {
		// No cookie means the client is already logged out — treat as success.
		h.clearRefreshCookie(w)
		common.WriteSuccess(w, r, http.StatusOK, nil)
		return
	}

	if err := h.svc.Logout(r.Context(), cookie.Value); err != nil {
		// Log but don't fail — clear the cookie regardless.
		slog.WarnContext(r.Context(), "logout db cleanup failed", "error", err)
	}

	h.clearRefreshCookie(w)
	common.WriteSuccess(w, r, http.StatusOK, nil)
}

// ── Cookie helpers ────────────────────────────────────────────────────────────

func (h *Handler) setRefreshCookie(w http.ResponseWriter, rawToken string) {
	http.SetCookie(w, &http.Cookie{
		Name:     refreshTokenCookie,
		Value:    rawToken,
		Path:     "/",
		Domain:   h.cfg.CookieDomain,
		Expires:  time.Now().Add(time.Duration(h.cfg.RefreshTokenExpiryDays) * 24 * time.Hour),
		HttpOnly: true,
		Secure:   h.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (h *Handler) clearRefreshCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     refreshTokenCookie,
		Value:    "",
		Path:     "/",
		Domain:   h.cfg.CookieDomain,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

// ── Validation ────────────────────────────────────────────────────────────────

func validateRegisterRequest(req registerRequest) *common.AppError {
	if !strings.Contains(req.Email, "@") {
		return common.BadRequest("INVALID_EMAIL", "a valid email address is required")
	}
	if len(req.Password) < 8 {
		return common.BadRequest("WEAK_PASSWORD", "password must be at least 8 characters")
	}
	if req.Role != "CLIENT" && req.Role != "PRO" {
		return common.BadRequest("INVALID_ROLE", "role must be CLIENT or PRO")
	}
	return nil
}
