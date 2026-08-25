package middleware

import "net/http"

// MismoOrigen rechaza peticiones cuyo Host no esté en hostsPermitidos. Cierra
// el DNS-rebinding: un dominio atacante que resuelve a 127.0.0.1 llega con su
// propio Host (evil.example), no con el loopback esperado, y se corta con 403.
// Lista vacía = deshabilitado (la suite httptest usa Host arbitrario).
func MismoOrigen(hostsPermitidos []string) func(http.Handler) http.Handler {
	permitido := make(map[string]struct{}, len(hostsPermitidos))
	for _, h := range hostsPermitidos {
		permitido[h] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if len(permitido) > 0 {
				if _, ok := permitido[r.Host]; !ok {
					http.Error(w, "origen no permitido", http.StatusForbidden)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}
