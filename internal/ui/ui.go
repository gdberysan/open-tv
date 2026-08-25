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

func (s *servidor) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
