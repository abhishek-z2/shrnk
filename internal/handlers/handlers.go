package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/abhishek-z2/shrnk/internal/auth"
	"github.com/abhishek-z2/shrnk/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	minURLLifetime = time.Minute
	maxURLifetime  = 30 * 24 * time.Hour
)

type URLStore interface {
	GetURL(ctx context.Context, shortCode string) (store.URLRecord, error)
	CreateURL(ctx context.Context, longURL string, expiresAt time.Time) (int64, string, error)
}

type URLCache interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value string, ttl time.Duration) error
}

type AuthStore interface {
	CreateUserWithSession(
		ctx context.Context,
		email string,
		passwordHash string,
		sessionID uuid.UUID,
		tokenHash string,
		expiresAt time.Time,
	) (store.User, error)
}

type Handler struct {
	store     URLStore
	cache     URLCache
	authStore AuthStore
}

func NewHandler(store URLStore, cache URLCache, authStore AuthStore) *Handler {
	return &Handler{
		store:     store,
		cache:     cache,
		authStore: authStore,
	}
}

type ShortenRequest struct {
	URL       string `json:"url"`
	ExpiresIn string `json:"expires_in"`
}

type ShortenResponse struct {
	ShortCode string `json:"short_code"`
}

type SignupRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *Handler) Shorten(w http.ResponseWriter, r *http.Request) {
	var req ShortenRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.URL == "" {
		http.Error(w, "url is required", http.StatusBadRequest)
		return
	}

	duration, err := time.ParseDuration(req.ExpiresIn)
	if err != nil {
		http.Error(w, "invalid expires_in", http.StatusBadRequest)
		return
	}

	if duration < minURLLifetime || duration > maxURLifetime {
		http.Error(w, "expires_in must be between 1 minute and 30 days", http.StatusBadRequest)
		return
	}

	expiresAt := time.Now().Add(duration)

	_, shortCode, err := h.store.CreateURL(r.Context(), req.URL, expiresAt)
	if err != nil {
		http.Error(w, "failed to create short URL", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ShortenResponse{
		ShortCode: shortCode,
	})
}

func (h *Handler) Redirect(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")

	longURL, err := h.cache.Get(r.Context(), code)
	if err == nil {
		http.Redirect(w, r, longURL, http.StatusFound)
		return
	}

	if !errors.Is(err, redis.Nil) {
		http.Error(w, "cache-error", http.StatusInternalServerError)
		return
	}

	var record store.URLRecord

	record, err = h.store.GetURL(r.Context(), code)

	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "failed to get URL", http.StatusInternalServerError)
		return
	}
	if record.ExpiresAt.Before(time.Now()) {
		http.Error(w, "code has expired", http.StatusBadRequest)
		return
	}

	ttl := time.Until(record.ExpiresAt)
	if err = h.cache.Set(r.Context(), code, record.LongURL, ttl); err != nil {
		log.Printf("failed to cache URL %q: %v", code, err)
	}
	http.Redirect(w, r, record.LongURL, http.StatusFound)
}

func (h *Handler) Signup(w http.ResponseWriter, r *http.Request) {
	var req SignupRequest

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // limit the body so that clients cant send arbitrarily large payloads (ddos attacks)

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	email := strings.ToLower(strings.TrimSpace(req.Email))
	if email == "" || !strings.Contains(email, "@") {
		http.Error(w, "valid email is required", http.StatusBadRequest)
		return
	}

	if len(req.Password) < 8 {
		http.Error(w, "password must be at least 8 characters", http.StatusBadRequest)
		return
	}

	passwordHash, err := auth.HashPassword(req.Password)
	if err != nil {
		log.Printf("hash password: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	token, err := auth.GenerateSessionToken()
	if err != nil {
		log.Printf("generate session token: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	const sessionLifetime = 7 * 24 * time.Hour
	sessionID := uuid.New()
	tokenHash := auth.HashSessionToken(token)
	expiresAt := time.Now().Add(sessionLifetime)

	_, err = h.authStore.CreateUserWithSession(
		r.Context(),
		email,
		passwordHash,
		sessionID,
		tokenHash,
		expiresAt,
	)

	if err != nil {
		log.Printf("create user with session: %v", err)
		http.Error(w, "could not create account", http.StatusConflict)
		return
	}

	http.SetCookie(
		w,
		&http.Cookie{
			Name:     "session",
			Value:    token,
			Path:     "/",
			HttpOnly: true,
			Secure:   false, //set true in production
			SameSite: http.SameSiteLaxMode,
			Expires:  expiresAt,
		})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]string{ // use map when sending one-off payloads (like alerts,succes messages)
		"message": "account created",
	})
}
