package auth

import (
	"context"
	"net/http"
	"time"

	"github.com/abhishek-z2/shrnk/internal/store"
)

type SessionStore interface {
	GetSession(ctx context.Context, tokenHash string) (store.Session, error)
}

func SessionMiddleware(db SessionStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie("session")
			if err != nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			tokenHash := HashSessionToken(cookie.Value)

			session, err := db.GetSession(r.Context(), tokenHash)
			if err != nil {
				if err == store.ErrNotFound {
					http.Error(w, "unauthorized", http.StatusUnauthorized)
					return
				}
				http.Error(w, "authentication failed", http.StatusInternalServerError)
			}

			if session.ExpiresAt.Before(time.Now()) {
				http.Error(w, "session expired", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(
				r.Context(),
				userIDKey,
				session.UserID,
			)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
