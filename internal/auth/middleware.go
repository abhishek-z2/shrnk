package auth

import (
	"github.com/abhishek-z2/shrnk/internal/store"
	"net/http"
)

func Middleware(db *store.PostgresStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.Header.Get("X-API-Key")

			if key == "" {
				http.Error(w, "missing API key", http.StatusUnauthorized)
				return
			}

			keyHash := HashKey(key)

			_, err := db.FindAPIKey(r.Context(), keyHash)
			if err != nil {
				if err == store.ErrNotFound {
					http.Error(w, "invalid API key", http.StatusUnauthorized)
					return
				}
				http.Error(w, "authentication failed", http.StatusInternalServerError)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
