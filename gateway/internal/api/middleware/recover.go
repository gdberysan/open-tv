package middleware

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// Recover es un middleware que captura panics y devuelve un error 500 JSON,
// evitando que el servidor se caiga.
func Recover(logger *slog.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					logger.Error("Panic capturado en handler",
						slog.Any("error", err),
						slog.String("path", r.URL.Path),
					)

					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
					_ = json.NewEncoder(w).Encode(map[string]string{
						"error": "Internal Server Error",
					})
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}
