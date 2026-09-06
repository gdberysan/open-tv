package validator

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/gdberysan/open-tv/internal/domain"
)

// maxBytesSonda es lo que se lee del primer segmento: 87 paquetes TS. Medido
// en orígenes reales, PAT y PMT van en los paquetes 2 y 3 (564 bytes).
const maxBytesSonda = 16 << 10

// maxBytesPlaylistSonda acota la media playlist intermedia (una playlist en
// vivo ronda el KB).
const maxBytesPlaylistSonda = 64 << 10

// sondearCodecs sigue el manifiesto hasta el primer segmento y lee su PMT.
// Devuelve CodecUnknown ante CUALQUIER problema: no poder leer no es "no
// sirve". Nunca lanza más de dos peticiones.
func (c *Checker) sondearCodecs(ctx context.Context, urlManifiesto, cuerpo, referrer, ua string) (domain.CodecSupport, string) {
	base, err := url.Parse(urlManifiesto)
	if err != nil {
		return domain.CodecUnknown, ""
	}
	media := cuerpo
	if esMaster(cuerpo) {
		u := resolverURI(base, primeraURI(cuerpo))
		if u == nil {
			return domain.CodecUnknown, ""
		}
		media, err = c.leerTexto(ctx, u, referrer, ua)
		if err != nil {
			return domain.CodecUnknown, ""
		}
		base = u
	}
	if strings.Contains(media, "#EXT-X-MAP") {
		// fMP4: el códec va en el init segment (moov/stsd). Fuera de alcance.
		return domain.CodecUnknown, ""
	}
	seg := resolverURI(base, primeraURI(media))
	if seg == nil {
		return domain.CodecUnknown, ""
	}
	prefijo, err := c.leerPrefijo(ctx, seg, referrer, ua)
	if err != nil || len(prefijo) == 0 || prefijo[0] != 0x47 {
		return domain.CodecUnknown, ""
	}
	streams, err := domain.ParsearPMT(prefijo)
	if err != nil {
		return domain.CodecUnknown, ""
	}
	return domain.ClassifyCodecs(streams), domain.NombreCodecs(streams)
}

func esMaster(cuerpo string) bool {
	return strings.Contains(cuerpo, "#EXT-X-STREAM-INF")
}

// primeraURI devuelve la primera línea que no es etiqueta ni está vacía: en
// un master es la primera variante, en una media playlist el primer segmento.
func primeraURI(cuerpo string) string {
	for _, linea := range strings.Split(cuerpo, "\n") {
		linea = strings.TrimSpace(linea)
		if linea == "" || strings.HasPrefix(linea, "#") {
			continue
		}
		return linea
	}
	return ""
}

func resolverURI(base *url.URL, uri string) *url.URL {
	if uri == "" {
		return nil
	}
	u, err := base.Parse(uri)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return nil
	}
	return u
}

func (c *Checker) peticionSonda(ctx context.Context, u *url.URL, referrer, ua string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	if ua == "" {
		ua = userAgentPorDefecto
	}
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Origin", domain.OrigenWeb)
	if referrer != "" {
		req.Header.Set("Referer", referrer)
	}
	return req, nil
}

func (c *Checker) leerTexto(ctx context.Context, u *url.URL, referrer, ua string) (string, error) {
	req, err := c.peticionSonda(ctx, u, referrer, ua)
	if err != nil {
		return "", err
	}
	resp, err := c.sonda.Do(req) //nolint:gosec // URL de manifiesto; va por el cliente guardado (proxy.NuevoClienteGuardado)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", errStatus(resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxBytesPlaylistSonda))
	return string(b), err
}

// leerPrefijo pide los primeros 16 KB por Range. Si el origen lo ignora y
// contesta 200 con el segmento entero, se lee igualmente solo el prefijo y
// se cierra la conexión (sin keep-alive, cerrar aborta la descarga).
func (c *Checker) leerPrefijo(ctx context.Context, u *url.URL, referrer, ua string) ([]byte, error) {
	req, err := c.peticionSonda(ctx, u, referrer, ua)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Range", "bytes=0-16383")
	resp, err := c.sonda.Do(req) //nolint:gosec // URL de manifiesto; va por el cliente guardado (proxy.NuevoClienteGuardado)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return nil, errStatus(resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxBytesSonda))
}

type errStatus int

func (e errStatus) Error() string { return "sonda: HTTP " + http.StatusText(int(e)) }
