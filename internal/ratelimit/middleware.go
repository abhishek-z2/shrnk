package ratelimit

import (
	"net/http"

	"github.com/abhishek-z2/shrnk/internal/auth"
)

func Middleware(limiter *RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			apiKeyID, ok := auth.GetAPIKeyID(r.Context())
			if !ok {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			key := "rate_limit:" + apiKeyID.String()

			allowed, err := limiter.Allow(
				r.Context(),
				key,
				5, //cap
				1, //refill@1token/sec
			)
			if err != nil {
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}

			if !allowed {
				http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}

}

