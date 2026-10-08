package handlers

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/abhishek-z2/shrnk/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
)

type fakeCache struct {
	url       string
	err       error
	setKey    string
	setValue  string
	setCalled bool
}

func (f *fakeCache) Get(ctx context.Context, key string) (string, error) {
	fmt.Println("fake cache error:", f.err)
	return f.url, f.err
}
func (f *fakeCache) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	f.setCalled = true
	f.setKey = key
	f.setValue = value
	return nil
}

type fakeStore struct {
	getURLCalled bool
	url          store.URLRecord
}

func (f *fakeStore) GetURL(ctx context.Context, shortCode string) (store.URLRecord, error) {
	f.getURLCalled = true
	return f.url, nil
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

func TestRedirect_CacheMiss(t *testing.T) {
	store := &fakeStore{
		url: store.URLRecord{
			LongURL:   "https://example.com",
			ExpiresAt: time.Now().Add(time.Hour),
		},
	}
	cache := &fakeCache{
		err: redis.Nil,
	}

	h := NewHandler(store, cache)

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
	if !store.getURLCalled {
		t.Error("expected store to be called on cache miss")
	}
	if !cache.setCalled {
		t.Error("expected setCalled to be called on cache miss")
	}
	if cache.setKey != "abc123" {
		t.Errorf("expected cache key %q, got %q", "abc123", cache.setKey)
	}

	if cache.setValue != "https://example.com" {
		t.Errorf("expected cache value %q, got %q", "https://example.com", cache.setValue)
	}
}
