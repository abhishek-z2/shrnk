package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/abhishek-z2/shrnk/internal/auth"
	"github.com/abhishek-z2/shrnk/internal/store"
	"github.com/google/uuid"
)

type meFakeSessionStore struct {
	session store.Session
	err     error
}

func (f *meFakeSessionStore) GetSession(
	ctx context.Context,
	tokenHash string,
) (store.Session, error) {
	if f.err != nil {
		return store.Session{}, f.err
	}
	return f.session, nil
}

func TestMe(t *testing.T) {
	const (
		userID = int64(42)
		email  = "test@example.com"
		token  = "test-session-token"
	)

	tests := []struct {
		name       string
		user       store.User
		userErr    error
		wantStatus int
		wantEmail  string
	}{
		{
			name: "returns authenticated user",
			user: store.User{
				ID:    userID,
				Email: email,
			},
			wantStatus: http.StatusOK,
			wantEmail:  email,
		},
		{
			name:       "user does not exist",
			userErr:    store.ErrNotFound,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "database failure",
			userErr:    errors.New("database unavailable"),
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authStore := &fakeAuthStore{
				getUserByIDFunc: func(
					ctx context.Context,
					id int64,
				) (store.User, error) {
					if id != userID {
						t.Errorf("GetUserByID() id = %d, want %d", id, userID)
					}
					return tt.user, tt.userErr
				},
			}

			sessionStore := &meFakeSessionStore{
				session: store.Session{
					ID:        uuid.New(),
					UserID:    userID,
					TokenHash: auth.HashSessionToken(token),
					ExpiresAt: time.Now().Add(time.Hour),
				},
			}

			h := NewHandler(nil, nil, authStore)

			handler := auth.SessionMiddleware(sessionStore)(
				http.HandlerFunc(h.Me),
			)

			req := httptest.NewRequest(
				http.MethodGet,
				"/api/auth/me",
				nil,
			)
			req.AddCookie(&http.Cookie{
				Name:  "session",
				Value: token,
			})

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf(
					"status = %d, want %d; body = %s",
					rec.Code,
					tt.wantStatus,
					rec.Body.String(),
				)
			}

			if tt.wantStatus != http.StatusOK {
				return
			}

			var got MeResponse
			if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
				t.Fatalf("decode response: %v", err)
			}

			if got.ID != userID {
				t.Errorf("ID = %d, want %d", got.ID, userID)
			}
			if got.Email != tt.wantEmail {
				t.Errorf("Email = %q, want %q", got.Email, tt.wantEmail)
			}
		})
	}
}
