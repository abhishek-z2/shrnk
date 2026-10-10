package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
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
	getURLCalled    bool
	createURLCalled bool
	url             store.URLRecord
	err             error

	createShortCode string
	createErr       error
	createLongURL   string
	createExpiresAt time.Time
}

func (f *fakeStore) GetURL(ctx context.Context, shortCode string) (store.URLRecord, error) {
	f.getURLCalled = true
	return f.url, f.err
}

func (f *fakeStore) CreateURL(ctx context.Context, longURL string, expiresAt time.Time) (int64, string, error) {
	f.createURLCalled = true
	f.createLongURL = longURL
	f.createExpiresAt = expiresAt
	return 0, f.createShortCode, f.createErr
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

func TestRedirect__NotFound(t *testing.T) {
	store := &fakeStore{
		err: store.ErrNotFound,
	}
	cache := &fakeCache{
		err: redis.Nil,
	}

	h := NewHandler(store, cache)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("code", "missing")

	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	req = req.WithContext(context.WithValue(
		req.Context(),
		chi.RouteCtxKey,
		rctx,
	))

	rec := httptest.NewRecorder()

	h.Redirect(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}

	if !store.getURLCalled {
		t.Error("expected store to be called when cache misses")
	}
}

func TestRedirect_ExpiredURL(t *testing.T) {
	store := &fakeStore{
		url: store.URLRecord{
			LongURL:   "https://example.com",
			ExpiresAt: time.Now().Add(-time.Minute),
		},
	}
	cache := &fakeCache{
		err: redis.Nil,
	}

	h := NewHandler(store, cache)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("code", "expired")

	req := httptest.NewRequest(http.MethodGet, "/expired", nil)
	req = req.WithContext(context.WithValue(
		req.Context(),
		chi.RouteCtxKey,
		rctx,
	))

	rec := httptest.NewRecorder()

	h.Redirect(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	if got := rec.Header().Get("Location"); got != "" {
		t.Errorf("expected no Location header, got %q", got)
	}

	if cache.setCalled {
		t.Error("expected expired URL not to be cached")
	}
}

func TestRedirect_CacheError(t *testing.T) {
	store := &fakeStore{}
	cache := &fakeCache{
		err: errors.New("redis connection failed"),
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

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}

	if store.getURLCalled {
		t.Error("expected store not to be called when cache returns an unexpected error")
	}
}

func TestShorten_InvalidJSON(t *testing.T) {
	store := &fakeStore{}
	cache := &fakeCache{}

	h := NewHandler(store, cache)

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/shorten",
		strings.NewReader(`{"url":`),
	)
	rec := httptest.NewRecorder()

	h.Shorten(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	if store.createURLCalled {
		t.Error("expected CreateURL not to be called for invalid JSON")
	}
}

func TestShorten_MissingURL(t *testing.T) {
	store := &fakeStore{}
	cache := &fakeCache{}

	h := NewHandler(store, cache)

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/shorten",
		strings.NewReader(`{"expires_in":"1h"}`),
	)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()

	h.Shorten(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	if store.createURLCalled {
		t.Error("expected CreateURL not to be called when URL is missing")
	}
}

func TestShorten_InvalidExpiresIn(t *testing.T) {
	store := &fakeStore{}
	cache := &fakeCache{}
	h := NewHandler(store, cache)

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/shorten",
		strings.NewReader(`{"url":"https://example.com","expires_in":"banana"}`),
	)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	h.Shorten(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	if store.createURLCalled {
		t.Error("expected CreateURL not to be called for invalid expires_in")
	}
}

func TestShorten_ExpiresInTooShort(t *testing.T) {
	store := &fakeStore{}
	cache := &fakeCache{}
	h := NewHandler(store, cache)

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/shorten",
		strings.NewReader(`{"url":"https://example.com","expires_in":"30s"}`),
	)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	h.Shorten(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	if store.createURLCalled {
		t.Error("expected CreateURL not to be called when expires_in is too short")
	}
}

func TestShorten_ExpiresInTooLong(t *testing.T) {
	store := &fakeStore{}
	cache := &fakeCache{}
	h := NewHandler(store, cache)

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/shorten",
		strings.NewReader(`{"url":"https://example.com","expires_in":"31d"}`),
	)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	h.Shorten(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	if store.createURLCalled {
		t.Error("expected CreateURL not to be called when expires_in is too long")
	}
}

func TestShorten_Success(t *testing.T) {
	store := &fakeStore{
		createShortCode: "abc123",
	}
	cache := &fakeCache{}
	h := NewHandler(store, cache)

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/shorten",
		strings.NewReader(
			`{"url":"https://example.com","expires_in":"1h"}`,
		),
	)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	h.Shorten(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	if !store.createURLCalled {
		t.Fatal("expected CreateURL to be called")
	}

	if store.createLongURL != "https://example.com" {
		t.Errorf("expected long URL %q, got %q",
			"https://example.com", store.createLongURL)
	}

	if store.createExpiresAt.Before(time.Now().Add(59*time.Minute)) ||
		store.createExpiresAt.After(time.Now().Add(61*time.Minute)) {
		t.Errorf("expected expiration roughly one hour from now, got %v",
			store.createExpiresAt)
	}

	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", got)
	}

	var response ShortenResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.ShortCode != "abc123" {
		t.Errorf("expected short code %q, got %q", "abc123", response.ShortCode)
	}
}

func TestShorten_StoreError(t *testing.T) {
	store := &fakeStore{
		createErr: errors.New("database connection failed"),
	}
	cache := &fakeCache{}
	h := NewHandler(store, cache)

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/shorten",
		strings.NewReader(
			`{"url":"https://example.com","expires_in":"1h"}`,
		),
	)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	h.Shorten(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d",
			http.StatusInternalServerError, rec.Code)
	}

	if !store.createURLCalled {
		t.Error("expected CreateURL to be called")
	}
}

/*func TestRedirect_ExpiredURL(t *testing.T) {

}*/
