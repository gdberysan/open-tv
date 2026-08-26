// Package ui sirve el cliente web embebido en el binario.
//
// El cliente se construye directamente en internal/ui/dist (build.outDir de
// Vite) y no en web/dist: go:embed no admite "..", así que el destino del
// build tiene que vivir junto al paquete que lo embebe.
package ui

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed all:dist
var dist embed.FS

// Handler devuelve el servidor del cliente y si de verdad hay cliente. Un
// binario compilado sin haber construido web/ sigue sirviendo la API: es un
// modo degradado legítimo (el LaunchAgent de desarrollo, por ejemplo), no un
// fallo fatal.
func Handler() (http.Handler, bool) {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		return nil, false
	}
	if _, err := fs.Stat(sub, "index.html"); err != nil {
		return nil, false
	}
	return &servidor{archivos: sub, ficheros: http.FileServerFS(sub)}, true
}

type servidor struct {
	archivos fs.FS
	ficheros http.Handler
}

// csp es la política de seguridad de contenido del cliente embebido. La pieza
// que de verdad protege es `script-src 'self'`: NO hay scripts inline en el
// index.html construido (Vite emite un único <script type=module src=...>), así
// que podemos prohibir todo script que no sea del propio binario sin
// concesiones — cierra el vector real de XSS aunque el catálogo es contenido
// de terceros no confiable. El resto es a propósito permisivo porque es la
// función de la app: los logos (`img-src`) y los streams (`media-src` +
// `connect-src` para los fetch de hls.js) vienen de cualquier host FTA, http o
// https; `blob:` lo necesita hls.js (MSE). `style-src` lleva 'unsafe-inline'
// porque Svelte emite atributos style="" (p.ej. las alturas de los
// espaciadores de la rejilla virtual); inyectar estilos es de bajo riesgo
// comparado con inyectar scripts. `frame-ancestors 'none'` + `object-src
// 'none'` + `base-uri 'self'` cierran clickjacking y trucos de base/objeto.
const csp = "default-src 'self'; " +
	"script-src 'self'; " +
	"style-src 'self' 'unsafe-inline'; " +
	"img-src 'self' data: https: http:; " +
	"media-src 'self' blob: https: http:; " +
	"connect-src 'self' https: http:; " +
	"font-src 'self'; " +
	"object-src 'none'; " +
	"base-uri 'self'; " +
	"frame-ancestors 'none'; " +
	"form-action 'self'"

// cabecerasSeguridad añade la CSP y las cabeceras de endurecimiento a toda
// respuesta del cliente. Defensa en profundidad (hallazgo M1 de la revisión de
// seguridad): hoy el cliente ya renderiza el catálogo no confiable de forma
// segura (Svelte escapa el texto, el <img src> es inerte, no hay {@html}), pero
// esto es la segunda capa si algún día se introduce un sink de XSS.
func cabecerasSeguridad(w http.ResponseWriter) {
	h := w.Header()
	h.Set("Content-Security-Policy", csp)
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("X-Frame-Options", "DENY") // redundante con frame-ancestors; cubre navegadores viejos
}

func (s *servidor) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	cabecerasSeguridad(w)
	limpia := path.Clean("/" + strings.TrimPrefix(r.URL.Path, "/"))

	if limpia == "/" {
		s.servirIndex(w, r)
		return
	}

	if _, err := fs.Stat(s.archivos, strings.TrimPrefix(limpia, "/")); err == nil {
		// Los assets de Vite llevan hash en el nombre: son inmutables por
		// construcción y cachearlos un año es gratis y correcto.
		if strings.HasPrefix(limpia, "/assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		s.ficheros.ServeHTTP(w, r)
		return
	}

	// Una ruta con extensión que no existe es un 404 de verdad: devolver el
	// index para /favicon.ico o /assets/roto.js esconde errores de red detrás
	// de HTML que el navegador no sabe interpretar.
	if path.Ext(limpia) != "" {
		http.NotFound(w, r)
		return
	}

	// Ruta del cliente: fallback SPA.
	s.servirIndex(w, r)
}

func (s *servidor) servirIndex(w http.ResponseWriter, r *http.Request) {
	datos, err := fs.ReadFile(s.archivos, "index.html")
	if err != nil {
		http.Error(w, "cliente web no disponible", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// no-cache y no no-store: el navegador puede guardarlo, pero tiene que
	// revalidarlo. Así un despliegue nuevo se ve al recargar.
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(datos)
}
