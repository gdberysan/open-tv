// Package iptvorg lee la API pública de iptv-org (streams.json y channels.json)
// para aportar lo que el index.m3u descarta: los mirrors extra de cada canal,
// las cabeceras que algunos orígenes exigen, y la categoría real de los canales
// que el M3U deja en «General».
//
// Es un colaborador OPCIONAL del proveedor opensource: sin él, ese proveedor se
// comporta exactamente igual que siempre, que es lo que necesitan los M3U que
// sube el usuario (no tienen API detrás).
package iptvorg

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	URLStreams  = "https://iptv-org.github.io/api/streams.json"
	URLChannels = "https://iptv-org.github.io/api/channels.json"

	// maxJSONBytes acota la descarga. Medido el 2026-09-04: streams.json pesa
	// ~3,5 MB y channels.json ~7,8 MB. 64 MB deja margen de sobra sin permitir
	// que un upstream roto agote la memoria.
	maxJSONBytes = 64 << 20

	// tiempoCarga cubre las dos descargas juntas.
	tiempoCarga = 2 * time.Minute
)

// StreamExtra es un stream de la API con las cabeceras que su origen exige.
// Referrer y UserAgent vacíos significan "usa los de siempre".
type StreamExtra struct {
	URL       string
	Referrer  string
	UserAgent string
}

// claveFeed identifica un feed concreto de un canal. El feed DISCRIMINA: el
// mismo canal tiene entradas distintas para HD y SD, y mezclarlas asigna al SD
// los streams del HD.
type claveFeed struct{ canal, feed string }

type Enriquecedor struct {
	client      *http.Client
	urlStreams  string
	urlChannels string

	mu         sync.RWMutex
	porFeed    map[claveFeed][]StreamExtra
	categorias map[string]string
}

func NuevoEnriquecedor(client *http.Client) *Enriquecedor {
	if client == nil {
		client = &http.Client{Timeout: tiempoCarga}
	}
	return &Enriquecedor{
		client:      client,
		urlStreams:  URLStreams,
		urlChannels: URLChannels,
		porFeed:     make(map[claveFeed][]StreamExtra),
		categorias:  make(map[string]string),
	}
}

// SepararTvgID parte el tvg_id del M3U en (canal, feed). El sufijo tras el
// PRIMER '@' es exactamente el campo `feed` de la API:
//
//	"AndTV.in@HD" -> ("AndTV.in", "HD")
//	"Solo.uk"     -> ("Solo.uk", "")
func SepararTvgID(tvgID string) (canal, feed string) {
	canal, feed, _ = strings.Cut(tvgID, "@")
	return canal, feed
}

// entradaStream refleja SOLO los campos que se usan de streams.json. El resto
// (title, quality, labels) se descarta al decodificar.
type entradaStream struct {
	Channel   *string `json:"channel"`
	Feed      *string `json:"feed"`
	URL       string  `json:"url"`
	Referrer  *string `json:"referrer"`
	UserAgent *string `json:"user_agent"`
}

// entradaCanal refleja SOLO lo que se usa de channels.json. OJO: este fichero
// NO tiene campo `languages` (comprobado sobre el JSON real), y pesa ~7,8 MB,
// así que se guarda únicamente la categoría y se tira el resto.
type entradaCanal struct {
	ID         string   `json:"id"`
	Categories []string `json:"categories"`
}

func valor(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// Cargar descarga e indexa los dos ficheros. Se llama UNA vez por sync, nunca
// por canal.
func (e *Enriquecedor) Cargar(ctx context.Context) error {
	var streams []entradaStream
	if err := e.descargarJSON(ctx, e.urlStreams, &streams); err != nil {
		return fmt.Errorf("iptvorg.Cargar (streams): %w", err)
	}
	var canales []entradaCanal
	if err := e.descargarJSON(ctx, e.urlChannels, &canales); err != nil {
		return fmt.Errorf("iptvorg.Cargar (channels): %w", err)
	}

	porFeed := make(map[claveFeed][]StreamExtra, len(streams))
	for _, s := range streams {
		// Sin `channel` no hay con qué ligarlo a nuestro catálogo: se descarta.
		if s.Channel == nil || *s.Channel == "" || s.URL == "" {
			continue
		}
		k := claveFeed{canal: *s.Channel, feed: valor(s.Feed)}
		porFeed[k] = append(porFeed[k], StreamExtra{
			URL:       s.URL,
			Referrer:  valor(s.Referrer),
			UserAgent: valor(s.UserAgent),
		})
	}

	categorias := make(map[string]string, len(canales))
	for _, c := range canales {
		if c.ID == "" || len(c.Categories) == 0 {
			continue
		}
		categorias[c.ID] = c.Categories[0]
	}

	e.mu.Lock()
	e.porFeed = porFeed
	e.categorias = categorias
	e.mu.Unlock()
	return nil
}

func (e *Enriquecedor) descargarJSON(ctx context.Context, url string, destino any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("construyendo peticion: %w", err)
	}
	resp, err := e.client.Do(req)
	if err != nil {
		return fmt.Errorf("descargando: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	// LimitReader con N+1 para distinguir "justo en el límite" de "excedido",
	// igual que hace el parser de M3U.
	limitado := &io.LimitedReader{R: resp.Body, N: maxJSONBytes + 1}
	cuerpo, err := io.ReadAll(limitado)
	if err != nil {
		return fmt.Errorf("leyendo cuerpo: %w", err)
	}
	if int64(len(cuerpo)) > maxJSONBytes {
		return fmt.Errorf("respuesta mayor que el limite de %d bytes", maxJSONBytes)
	}
	if err := json.Unmarshal(cuerpo, destino); err != nil {
		return fmt.Errorf("decodificando JSON: %w", err)
	}
	return nil
}

// Streams devuelve los streams del par exacto (canal, feed), en el orden en que
// venían en la API. nil si no hay ninguno.
func (e *Enriquecedor) Streams(canal, feed string) []StreamExtra {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.porFeed[claveFeed{canal: canal, feed: feed}]
}

// Categoria devuelve la PRIMERA categoría upstream del canal. Se usa solo para
// rellenar las categorías débiles del M3U (ver el syncer).
func (e *Enriquecedor) Categoria(canal string) (string, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	c, ok := e.categorias[canal]
	return c, ok
}

// totalIndexados es para tests: cuántos streams quedaron en el índice.
func (e *Enriquecedor) totalIndexados() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	n := 0
	for _, v := range e.porFeed {
		n += len(v)
	}
	return n
}
