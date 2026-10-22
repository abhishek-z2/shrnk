package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/abhishek-z2/shrnk/internal/store"
	"github.com/google/uuid"
)

type fakeSessionStore struct {
	getSessionFunc func(
		ctx context.Context,
		tokenHash string,
	) (store.Session, error)
}

func (f *fakeSessionStore) GetSession(
	ctx context.Context,
	tokenHash string,
) (store.Session, error) {
	if f.getSessionFunc == nil {
		return store.Session{}, store.ErrNotFound
	}

	return f.getSessionFunc(ctx, tokenHash)
}

func TestSessionMiddleware_MissingCookie(t *testing.T) {
	store := &fakeSessionStore{}

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called")
	})

	handler := SessionMiddleware(store)(next)

	req := httptest.NewRequest(http.MethodGet, "/api/urls", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d",
			rec.Code,
			http.StatusUnauthorized,
		)
	}
}

func TestSessionMiddleware_Authentication(t *testing.T) {
	const token = "test-session-token"

	validSession := store.Session{
		ID:        uuid.New(),
		UserID:    42,
		TokenHash: HashSessionToken(token),
		ExpiresAt: time.Now().Add(time.Hour),
		CreatedAt: time.Now(),
	}

	tests := []struct {
		name           string
		session        store.Session
		storeErr       error
		wantStatus     int
		wantNextCalled bool
		wantUserID     int64
	}{
		{
			name:       "invalid session token",
			storeErr:   store.ErrNotFound,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name: "expired session",
			session: store.Session{
				ID:        uuid.New(),
				UserID:    42,
				TokenHash: HashSessionToken(token),
				ExpiresAt: time.Now().Add(-time.Hour),
			},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "database failure",
			storeErr:   errors.New("database unavailable"),
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:           "valid session",
			session:        validSession,
			wantStatus:     http.StatusOK,
			wantNextCalled: true,
			wantUserID:     42,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeStore := &fakeSessionStore{
				getSessionFunc: func(
					ctx context.Context,
					tokenHash string,
				) (store.Session, error) {
					if tokenHash != HashSessionToken(token) {
						t.Fatalf("unexpected token hash")
					}
					return tt.session, tt.storeErr
				},
			}

			nextCalled := false

			next := http.HandlerFunc(func(
				w http.ResponseWriter,
				r *http.Request,
			) {
				nextCalled = true

				userID, ok := GetUserID(r.Context())
				if !ok {
					t.Error("user ID missing from request context")
					return
				}

				if userID != tt.wantUserID {
					t.Errorf(
						"user ID = %d, want %d",
						userID,
						tt.wantUserID,
					)
				}

				w.WriteHeader(http.StatusOK)
			})

			handler := SessionMiddleware(fakeStore)(next)

			req := httptest.NewRequest(
				http.MethodGet,
				"/api/urls",
				nil,
			)
			req.AddCookie(&http.Cookie{
				Name:  "session",
				Value: token,
			})

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf(
					"status = %d, want %d; body = %s",
					rec.Code,
					tt.wantStatus,
					rec.Body.String(),
				)
			}

			if nextCalled != tt.wantNextCalled {
				t.Errorf(
					"nextCalled = %v, want %v",
					nextCalled,
					tt.wantNextCalled,
				)
			}
		})
	}
}
