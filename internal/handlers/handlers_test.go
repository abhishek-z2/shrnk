package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/abhishek-z2/shrnk/internal/store"
	"github.com/go-chi/chi/v5"
)

type fakeCache struct {
	url string
	err error
}

func (f *fakeCache) Get(ctx context.Context, key string) (string, error) {
	return f.url, f.err
}
func (f *fakeCache) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	return nil
}

type fakeStore struct {
	getURLCalled bool
}

func (f *fakeStore) GetURL(ctx context.Context, shortCode string) (store.URLRecord, error) {
	f.getURLCalled = true
	return store.URLRecord{}, errors.New("store should not have been called")
}

func (f *fakeStore) CreateURL(ctx context.Context, longURL string, expiresAt time.Time) (int64, string, error) {
	return 0, "", nil
}

func TestRedirect_CacheHit(t *testing.T) {
	cache := &fakeCache{
		url: "https://example.com",
	}
	store := &fakeStore{}

	h := NewHandler(nil, cache)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("code", "abc123")

	req := httptest.NewRequest(http.MethodGet, "/abc123", nil)
	req = req.WithContext(context.WithValue(
		req.Context(),
		chi.RouteCtxKey,
		rctx,
	))

	rec := httptest.NewRecorder()

	h.Redirect(rec, req)

	if rec.Code != http.StatusFound {
		t.Errorf("expected status %d, got %d", http.StatusFound, rec.Code)
	}
	if got := rec.Header().Get("Location"); got != "https://example.com" {
		t.Errorf("expected Location %q, got %q", "https://example.com", got)
	}
	if store.getURLCalled {
		t.Error("expected store not to be called on cache hit")
	}
}
