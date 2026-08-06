package middleware

import (
	"net/http"
)

// RateLimiter básico basado en concurrencia (Semaphore).
// Evita el ahogamiento por ráfagas excesivas.
func RateLimiter(maxConcurrent int) func(next http.Handler) http.Handler {
	sem := make(chan struct{}, maxConcurrent)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Intentar adquirir un token
			select {
			case sem <- struct{}{}:
				// Token adquirido
				defer func() { <-sem }()
				next.ServeHTTP(w, r)
			default:
				// Sin tokens disponibles (exceso de concurrencia)
				http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			}
		})
	}
}
