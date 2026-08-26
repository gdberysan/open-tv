package handlers

import (
	"net/http"
	"net/url"

	"github.com/go-chi/chi/v5"
)

// pathParam devuelve un path param des-escapado. chi entrega el valor aún
// escapado cuando la URL original conserva RawPath (p.ej. IDs de canal como
// "BBC%20Alba%20(1080p)"). Si el des-escape falla — un nombre con "%"
// literal — se devuelve el valor tal cual.
func pathParam(r *http.Request, key string) string {
	v := chi.URLParam(r, key)
	if unescaped, err := url.PathUnescape(v); err == nil {
		return unescaped
	}
	return v
}
