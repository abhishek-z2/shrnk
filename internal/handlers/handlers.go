package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/abhishek-z2/shrnk/internal/store"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	store *store.PostgresStore
}

func NewHandler(store *store.PostgresStore) *Handler {
	return &Handler{
		store: store,
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
	longURL, err := h.store.GetURL(r.Context(), code)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "failed to get URL", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, longURL, http.StatusFound)
}
