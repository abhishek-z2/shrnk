package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/abhishek-z2/shrnk/internal/cache"
	"github.com/abhishek-z2/shrnk/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
)

type RedisCache struct {
	client *redis.Client
}

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
	URL string `json:"url"`
}

type ShortenResponse struct {
	ShortCode string `json:"short_code"`
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

	_, shortCode, err := h.store.CreateURL(r.Context(), req.URL)
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

	longURL, err = h.store.GetURL(r.Context(), code)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "failed to get URL", http.StatusInternalServerError)
		return
	}
	if err = h.cache.Set(r.Context(), code, longURL); err != nil {
		log.Printf("failed to cache URL %q: %v", code, err)
	}
	http.Redirect(w, r, longURL, http.StatusFound)
}
