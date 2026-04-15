package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/taskflow/backend/internal/model"
	"github.com/taskflow/backend/internal/store"
	"golang.org/x/crypto/bcrypt"
)

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

type AuthHandler struct {
	users     *store.UserStore
	jwtSecret []byte
}

func NewAuthHandler(users *store.UserStore, jwtSecret string) *AuthHandler {
	return &AuthHandler{
		users:     users,
		jwtSecret: []byte(jwtSecret),
	}
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req model.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if errs := req.Validate(); len(errs) > 0 {
		writeValidationError(w, errs)
		return
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), 12)
	if err != nil {
		slog.Error("bcrypt hash failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	user, err := h.users.Create(r.Context(), strings.TrimSpace(req.Name), normalizeEmail(req.Email), string(hashed))
	if err != nil {
		if errors.Is(err, store.ErrEmailTaken) {
			writeValidationError(w, map[string]string{"email": "already taken"})
			return
		}
		slog.Error("create user failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	token, err := h.generateToken(user)
	if err != nil {
		slog.Error("token generation failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusCreated, model.AuthResponse{Token: token, User: *user})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req model.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if errs := req.Validate(); len(errs) > 0 {
		writeValidationError(w, errs)
		return
	}

	user, err := h.users.GetByEmail(r.Context(), normalizeEmail(req.Email))
	if err != nil {
		if errors.Is(err, store.ErrUserNotFound) {
			writeError(w, http.StatusUnauthorized, "invalid credentials")
			return
		}
		slog.Error("get user failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	token, err := h.generateToken(user)
	if err != nil {
		slog.Error("token generation failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, model.AuthResponse{Token: token, User: *user})
}

func (h *AuthHandler) generateToken(user *model.User) (string, error) {
	claims := jwt.MapClaims{
		"user_id": user.ID,
		"email":   user.Email,
		"exp":     time.Now().Add(24 * time.Hour).Unix(),
		"iat":     time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(h.jwtSecret)
}
