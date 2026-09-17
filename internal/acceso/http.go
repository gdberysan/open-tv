package acceso

import (
	"net"
	"net/http"
	"strings"
)

// NombreCookie y RutaAcceso son públicos para que el router y las pruebas no
// repitan literales.
const (
	NombreCookie = "open_tv_sesion"
	RutaAcceso   = "/acceso"
)

// maxCuerpoAcceso: un formulario con un campo no necesita más.
const maxCuerpoAcceso = 4 << 10

// Exigir deja pasar solo peticiones con sesión válida, salvo /health (lo usan
// el HEALTHCHECK de Docker y la detección de instancia viva) y la propia
// página de acceso.
func Exigir(s *Sesiones) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if exenta(r) || conSesion(s, r) {
				next.ServeHTTP(w, r)
				return
			}
			if r.Method == http.MethodGet && strings.Contains(r.Header.Get("Accept"), "text/html") {
				http.Redirect(w, r, RutaAcceso, http.StatusSeeOther)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"acceso requerido"}`))
		})
	}
}

// exenta compara Path, pero chi enruta por RawPath cuando está fijado (una
// petición con % escapes en la ruta, tipo "/acces%6F"): con solo el chequeo
// de Path, Exigir la daba por exenta y chi no la encontraba como ruta propia,
// así que caía sin sesión al fallback SPA. Exigir RawPath == "" cierra ese
// hueco: una ruta exenta de verdad nunca llega con % escapes de por medio.
func exenta(r *http.Request) bool {
	if r.URL.RawPath != "" {
		return false
	}
	if r.URL.Path == RutaAcceso {
		return true
	}
	return r.URL.Path == "/health" && (r.Method == http.MethodGet || r.Method == http.MethodHead)
}

func conSesion(s *Sesiones, r *http.Request) bool {
	c, err := r.Cookie(NombreCookie)
	return err == nil && s.Valida(c.Value)
}

// Pagina sirve GET y POST de /acceso.
func Pagina(s *Sesiones, l *Limitador) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead:
			if conSesion(s, r) {
				http.Redirect(w, r, "/", http.StatusSeeOther)
				return
			}
			pintar(w, r, http.StatusOK, false)
		case http.MethodPost:
			entrar(s, l, w, r)
		default:
			w.Header().Set("Allow", "GET, POST")
			http.Error(w, "método no permitido", http.StatusMethodNotAllowed)
		}
	})
}

func entrar(s *Sesiones, l *Limitador, w http.ResponseWriter, r *http.Request) {
	ip := ipDe(r)
	if l.Agotado(ip) {
		http.Error(w, "demasiados intentos; espera un minuto", http.StatusTooManyRequests)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxCuerpoAcceso)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "formulario inválido", http.StatusBadRequest)
		return
	}
	// La clave guardada ya está recortada al cargarla; pegar una clave con un
	// espacio final es habitual y no debe fallar el login por eso.
	if !s.ClaveCorrecta(strings.TrimSpace(r.PostForm.Get("clave"))) {
		// Solo los fallos gastan cupo: un login correcto (o varios dispositivos
		// detrás del mismo NAT entrando cada uno con éxito) no debe acercar a
		// nadie al 429.
		l.Fallo(ip)
		pintar(w, r, http.StatusUnauthorized, true)
		return
	}
	token, err := s.Emitir()
	if err != nil {
		http.Error(w, "no se pudo crear la sesión", http.StatusInternalServerError)
		return
	}
	//nolint:gosec // G124: HttpOnly y SameSite van literales; Secure se calcula
	// a propósito (detrás de un reverse-proxy TLS la petición llega en http),
	// nunca se omite.
	http.SetCookie(w, &http.Cookie{
		Name:     NombreCookie,
		Value:    token,
		Path:     "/",
		MaxAge:   int(DuracionSesion.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		// Detrás de un reverse proxy con TLS la petición llega en http: se mira
		// X-Forwarded-Proto. Falsificarlo solo sirve para que el propio
		// navegador del atacante no devuelva la cookie.
		Secure: r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https"),
	})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// ipDe usa RemoteAddr y no X-Forwarded-For: detrás de un reverse proxy todos
// comparten IP y el límite es global, que es peor para el uso pero no abre
// la puerta a saltárselo falsificando una cabecera.
func ipDe(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
