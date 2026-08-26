package middleware

import "net/http"

// MismoOrigen rechaza peticiones cuyo Host no esté en hostsPermitidos. Cierra
// el DNS-rebinding: un dominio atacante que resuelve a 127.0.0.1 llega con su
// propio Host (evil.example), no con el loopback esperado, y se corta con 403.
// Lista vacía = ese chequeo de Host queda deshabilitado (la suite httptest usa
// Host arbitrario).
//
// Además, para métodos MUTANTES (todo salvo GET/HEAD), exige Sec-Fetch-Site
// same-origin/none/ausente — el mismo guard y la misma justificación que ya
// documenta proxy/handler.go: esa cabecera la manda el navegador y el
// atacante no puede falsificarla desde JS, así que una petición cross-site
// (p.ej. un fetch POST simple, sin preflight, desde una página maliciosa
// hacia /sources) se corta aquí aunque el Host ya sea el nuestro. A
// diferencia del chequeo de Host, este SIEMPRE se aplica —incluso con
// hostsPermitidos vacío— porque no es un guard basado en host sino en
// origen: deshabilitarlo dejaría /sources (POST/PUT/PATCH/DELETE) abierto a
// CSRF en cualquier build que no configure HostsPermitidos.
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
			if r.Method != http.MethodGet && r.Method != http.MethodHead {
				if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" && site != "none" {
					http.Error(w, "origen cruzado no permitido", http.StatusForbidden)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}
