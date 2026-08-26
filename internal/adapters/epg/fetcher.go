package epg

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// defaultFetchTimeout cubre la descarga completa de una guía XMLTV (puede
// llegar a varios MB comprimidos). Mismo orden de magnitud que
// defaultClientTimeout del provider M3U (ver
// internal/adapters/providers/opensource/provider.go).
const defaultFetchTimeout = 5 * time.Minute

// Fetcher descarga una guía XMLTV desde la URL que la propia fuente (M3U)
// declaró (url-tvg / x-tvg-url). Es el mismo nivel de confianza que el fetch
// del M3U de la fuente (getLiveChannelsFromHTTP en el provider opensource):
// un cliente HTTP con timeout, sin guarda SSRF de red — a diferencia del
// proxy loopback estricto (internal/proxy/handler.go), que sí controla el
// Dial porque expone streams arbitrarios a un origen no autenticado. Aquí la
// URL viene de la playlist que el propio usuario configuró como fuente, así
// que no hay una frontera de confianza distinta a la que ya cruza el M3U.
type Fetcher struct {
	client *http.Client
}

// NewFetcher construye un Fetcher. Con client nil usa un *http.Client con
// defaultFetchTimeout, igual que NewProvider hace para el M3U.
func NewFetcher(client *http.Client) *Fetcher {
	if client == nil {
		client = &http.Client{Timeout: defaultFetchTimeout}
	}
	return &Fetcher{client: client}
}

// Descargar obtiene el cuerpo de url y devuelve un io.ReadCloser de XML en
// claro. Si el cuerpo viene gzip — por Content-Encoding: gzip, o porque la
// propia url termina en .gz/.xml.gz (el caso típico de una guía servida como
// fichero estático comprimido, sin que el servidor declare
// Content-Encoding) — se descomprime en streaming con compress/gzip; el
// llamador nunca tiene que distinguir el caso.
//
// El timeout de la descarga lo aplica el cliente HTTP (ver defaultFetchTimeout
// / el timeout que traiga client); el cap de TAMAÑO lo aplica ParsearXMLTV
// sobre el reader devuelto aquí — Descargar deliberadamente no trunca nada,
// para no enmascarar el error de tope bajo un EOF prematuro silencioso.
func (f *Fetcher) Descargar(ctx context.Context, url string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("epg.Descargar (NewRequest): %w", err)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("epg.Descargar (Do): %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("epg.Descargar (Status): HTTP %d en %s", resp.StatusCode, url)
	}

	if !esGzip(resp, url) {
		return resp.Body, nil
	}

	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("epg.Descargar (gzip): %w", err)
	}
	return &gzipReadCloser{gz: gz, body: resp.Body}, nil
}

// esGzip decide si el cuerpo de resp está comprimido con gzip, ya sea porque
// el servidor lo declaró explícitamente en la cabecera (nótese que el
// Transport de Go ya descomprime en claro, y retira la cabecera, el caso de
// negociación transparente vía Accept-Encoding automático — esta rama solo
// dispara cuando el servidor comprimió sin que se lo pidiéramos así) o
// porque la URL termina en una extensión de fichero gzip habitual.
func esGzip(resp *http.Response, url string) bool {
	if strings.EqualFold(resp.Header.Get("Content-Encoding"), "gzip") {
		return true
	}
	lower := strings.ToLower(url)
	return strings.HasSuffix(lower, ".gz")
}

// gzipReadCloser combina el *gzip.Reader con el io.ReadCloser del cuerpo
// HTTP subyacente para que un único Close() libere ambos recursos.
type gzipReadCloser struct {
	gz   *gzip.Reader
	body io.ReadCloser
}

func (g *gzipReadCloser) Read(p []byte) (int, error) {
	return g.gz.Read(p)
}

func (g *gzipReadCloser) Close() error {
	gzErr := g.gz.Close()
	bodyErr := g.body.Close()
	if gzErr != nil {
		return gzErr
	}
	return bodyErr
}
