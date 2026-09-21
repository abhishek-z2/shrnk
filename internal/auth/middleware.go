package auth

import (
	"context"
	"net/http"

	"github.com/abhishek-z2/shrnk/internal/store"
	"github.com/google/uuid"
)

type contextKey string

const apiKeyIDKey contextKey = "apiKeyID"

func Middleware(db *store.PostgresStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.Header.Get("X-API-Key")

			if key == "" {
				http.Error(w, "missing API key", http.StatusUnauthorized)
				return
			}

			keyHash := HashKey(key)

			apiKeyID, err := db.FindAPIKey(r.Context(), keyHash)
			if err != nil {
				if err == store.ErrNotFound {
					http.Error(w, "invalid API key", http.StatusUnauthorized)
					return
				}
				http.Error(w, "authentication failed", http.StatusInternalServerError)
				return
			}

			ctx := context.WithValue(r.Context(), apiKeyIDKey, apiKeyID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GetAPIKeyID(ctx context.Context) (uuid.UUID, bool) {
	value := ctx.Value(apiKeyIDKey)

	apiKeyID, ok := value.(uuid.UUID)
	return apiKeyID, ok
}
