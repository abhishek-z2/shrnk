package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/abhishek-z2/shrnk/internal/cache"
	"github.com/abhishek-z2/shrnk/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
)

const (
	minURLLifetime = time.Minute
	maxURLifetime  = 30 * 24 * time.Hour
)

type Handler struct {
	store *store.PostgresStore
	cache *cache.RedisCache
}

func NewHandler(store *store.PostgresStore, cache *cache.RedisCache) *Handler {
	return &Handler{
		store: store,
		cache: cache,
	}
}

type ShortenRequest struct {
	URL       string `json:"url"`
	ExpiresIn string `json:"expires_in"`
}

type ShortenResponse struct {
	ShortCode string `json:"short_code"`
}

type URLRecord struct {
	ID        int64
	ShortCode string
	LongURL   string
	ExpiresAt time.Time
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

	expires_at := time.Now().Add(duration)

	_, shortCode, err := h.store.CreateURL(r.Context(), req.URL, expires_at)
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
