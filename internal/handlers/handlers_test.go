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
	"github.com/google/uuid"
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

	h := NewHandler(nil, cache, nil)

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

	h := NewHandler(store, cache, nil)

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

	h := NewHandler(store, cache, nil)

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

	h := NewHandler(store, cache, nil)

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

	h := NewHandler(store, cache, nil)

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

	h := NewHandler(store, cache, nil)

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

	h := NewHandler(store, cache, nil)

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
	h := NewHandler(store, cache, nil)

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
	h := NewHandler(store, cache, nil)

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
	h := NewHandler(store, cache, nil)

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
	h := NewHandler(store, cache, nil)

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
	h := NewHandler(store, cache, nil)

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

type fakeAuthStore struct {
	createUserWithSessionFunc func(
		ctx context.Context,
		email string,
		passwordHash string,
		sessionID uuid.UUID,
		tokenHash string,
		expiresAt time.Time,
	) (store.User, error)
}

func (f *fakeAuthStore) CreateUserWithSession(
	ctx context.Context,
	email string,
	passwordHash string,
	sessionID uuid.UUID,
	tokenHash string,
	expiresAt time.Time,
) (store.User, error) {
	return f.createUserWithSessionFunc(
		ctx,
		email,
		passwordHash,
		sessionID,
		tokenHash,
		expiresAt,
	)
}

func TestHandler_Signup(t *testing.T) {
	var receivedEmail string
	var receivedPasswordHash string
	var receivedSessionID uuid.UUID
	var receivedTokenHash string
	var receivedExpiresAt time.Time

	authStore := &fakeAuthStore{
		createUserWithSessionFunc: func(
			ctx context.Context,
			email string,
			passwordHash string,
			sessionID uuid.UUID,
			tokenHash string,
			expiresAt time.Time,
		) (store.User, error) {
			receivedEmail = email
			receivedPasswordHash = passwordHash
			receivedSessionID = sessionID
			receivedTokenHash = tokenHash
			receivedExpiresAt = expiresAt

			return store.User{
				ID:           1,
				Email:        email,
				PasswordHash: passwordHash,
				CreatedAt:    time.Now(),
			}, nil
		},
	}

	h := NewHandler(nil, nil, authStore)

	body := `{"email":"test@example.com","password":"test-password-123"}`

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/auth/signup",
		strings.NewReader(body),
	)

	rec := httptest.NewRecorder()

	h.Signup(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", rec.Code)
	}

	if receivedEmail != "test@example.com" {
		t.Errorf(
			"expected email %q, got %q",
			"test@example.com",
			receivedEmail,
		)
	}

	// The plaintext password must never reach the store.
	if receivedPasswordHash == "test-password-123" {
		t.Error("password was stored as plaintext")
	}

	if receivedPasswordHash == "" {
		t.Error("expected password hash to be provided")
	}

	if receivedSessionID == uuid.Nil {
		t.Error("expected session ID to be generated")
	}

	if receivedTokenHash == "" {
		t.Error("expected session token hash to be provided")
	}

	if receivedExpiresAt.Before(time.Now()) {
		t.Error("expected session expiration to be in the future")
	}

	// Check the cookie returned to the browser.
	cookies := rec.Result().Cookies()

	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}

	cookie := cookies[0]

	if cookie.Name != "session" {
		t.Errorf("expected cookie name %q, got %q", "session", cookie.Name)
	}

	if cookie.Value == "" {
		t.Error("expected session cookie to contain a token")
	}

	if !cookie.HttpOnly {
		t.Error("expected session cookie to be HttpOnly")
	}

	if cookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("expected SameSite=Lax, got %v", cookie.SameSite)
	}

	if cookie.Path != "/" {
		t.Errorf("expected cookie path /, got %q", cookie.Path)
	}
}

func TestHandler_SignupRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "invalid JSON",
			body: `{"email":`,
		},
		{
			name: "missing email",
			body: `{"password":"test-password-123"}`,
		},
		{
			name: "invalid email",
			body: `{"email":"not-an-email","password":"test-password-123"}`,
		},
		{
			name: "short password",
			body: `{"email":"test@example.com","password":"short"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storeCalled := false

			authStore := &fakeAuthStore{
				createUserWithSessionFunc: func(
					ctx context.Context,
					email, passwordHash string,
					sessionID uuid.UUID,
					tokenHash string,
					expiresAt time.Time,
				) (store.User, error) {
					storeCalled = true
					return store.User{}, nil
				},
			}

			h := NewHandler(nil, nil, authStore)
			req := httptest.NewRequest(
				http.MethodPost,
				"/api/auth/signup",
				strings.NewReader(tt.body),
			)
			rec := httptest.NewRecorder()

			h.Signup(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Errorf(
					"expected status 400, got %d",
					rec.Code,
				)
			}

			// Invalid requests should never reach the database.
			if storeCalled {
				t.Error("store was called for invalid input")
			}
		})
	}
}

func TestHandler_SignupDuplicateEmail(t *testing.T) {
	authStore := &fakeAuthStore{
		createUserWithSessionFunc: func(
			ctx context.Context,
			email, passwordHash string,
			sessionID uuid.UUID,
			tokenHash string,
			expiresAt time.Time,
		) (store.User, error) {
			return store.User{}, errors.New("duplicate email")
		},
	}

	h := NewHandler(nil, nil, authStore)

	body := `{"email":"test@example.com","password":"test-password-123"}`
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/auth/signup",
		strings.NewReader(body),
	)
	rec := httptest.NewRecorder()

	h.Signup(rec, req)

	if rec.Code != http.StatusConflict {
		t.Errorf("expected status 409, got %d", rec.Code)
	}
}
