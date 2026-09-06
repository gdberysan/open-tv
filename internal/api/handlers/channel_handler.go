package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gdberysan/open-tv/internal/domain"
	"github.com/gdberysan/open-tv/internal/ports"
)

type ChannelHandler struct {
	logger   *slog.Logger
	repo     ports.ChannelRepository
	provider ports.ProviderPort
	streams  ports.StreamRepository
	prober   *AirplayProber
}

func NewChannelHandler(logger *slog.Logger, repo ports.ChannelRepository, provider ports.ProviderPort, streams ports.StreamRepository, prober *AirplayProber) *ChannelHandler {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	return &ChannelHandler{logger: logger, repo: repo, provider: provider, streams: streams, prober: prober}
}

// GetChannels aplica filtros combinados y devuelve canales paginados.
//
// Query params:
//   - q        búsqueda por nombre (LIKE)
//   - country  ISO 3166-1 alpha-2 (ej: GB, FR, DE)
//   - category ID exacto de categoría (ej: News, Sports)
//   - quality  fhd (1080p+, default) | hd (720p+) | 4k | all
//   - limit    default 500, max 1000
//   - offset   default 0
func (h *ChannelHandler) GetChannels(w http.ResponseWriter, r *http.Request) {
	quality := r.URL.Query().Get("quality")
	if strings.EqualFold(quality, "all") {
		quality = ""
	}

	// AliveOnly por defecto (Fase 7): los canales cuyos streams están todos
	// chequeados y muertos se ocultan salvo ?alive=all (o false/0).
	alive := r.URL.Query().Get("alive")
	aliveOnly := !strings.EqualFold(alive, "all") &&
		!strings.EqualFold(alive, "false") && alive != "0"

	filtro := filtroDesdeQuery(r)
	filtro.MinQuality = quality
	filtro.AliveOnly = aliveOnly
	filtro.Limit = queryInt(r, "limit", 500, 1000)
	filtro.Offset = queryInt(r, "offset", 0, -1)

	channels, err := h.repo.FindFiltered(r.Context(), filtro)
	if err != nil {
		h.logger.Error("GetChannels: fallo consultando el catálogo", slog.Any("error", err))
		h.writeError(w, http.StatusInternalServerError, "Error obteniendo canales")
		return
	}
	// Total real de coincidencias, para que la app muestre un contador veraz en
	// vez del número de canales que lleva cargados. Va en cabecera y no en el
	// cuerpo para no tocar el contrato JSON congelado por domain/channel_test.go.
	if total, err := h.repo.CountFiltered(r.Context(), filtro); err == nil {
		w.Header().Set("X-Total-Count", strconv.Itoa(total))
	} else {
		h.logger.Error("GetChannels: fallo contando el total", slog.Any("error", err))
	}

	h.writeJSON(w, http.StatusOK, channels)
}

// GetStreamURL resuelve la URL de reproducción para un canal.
// Ruta: GET /channels/stream?id=<channelID>
// Usa query param para soportar IDs con "/" (ej: "24/7 News").
//
// airplay_ok viaja aquí y no en /channels porque el sondeo cuesta una petición
// al origen: se paga solo por los canales que de verdad se van a reproducir.
// null significa "no se pudo determinar", que es el caso más común.
func (h *ChannelHandler) GetStreamURL(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		h.writeError(w, http.StatusBadRequest, "id requerido")
		return
	}
	url, err := h.resolveStreamURL(r, domain.ChannelID(id))
	if err != nil {
		h.writeError(w, http.StatusNotFound, "Stream no encontrado")
		return
	}

	respuesta := map[string]any{"url": url}
	if h.prober != nil {
		respuesta["airplay_ok"] = aBool(h.prober.Veredicto(r.Context(), url))
	}
	h.writeJSON(w, http.StatusOK, respuesta)
}

// resolveStreamURL elige qué URL servir, de mejor a peor fuente.
//
// La DB va primero a propósito. La caché del provider es un map[ChannelID]string
// poblado en el sync: una sola URL por canal, la última que gana si dos entradas
// del M3U comparten nombre, y sin ninguna noción de salud ni latencia. La DB, en
// cambio, guarda todos los streams del canal y el health-worker mantiene ahí
// is_alive y latency_ms. Consultar la caché primero significaba tirar esa
// información a la basura y poder devolver un stream muerto teniendo uno vivo al
// lado.
func (h *ChannelHandler) resolveStreamURL(r *http.Request, id domain.ChannelID) (string, error) {
	// 1. El mejor: vivo y de menor latencia.
	if best, err := h.streams.FindBestByChannelID(r.Context(), id); err == nil {
		return best.URL, nil
	}

	// 2. Ninguno vivo. Puede ser que el health-worker aún no haya pasado, así
	//    que servimos cualquiera antes que dar un 404 gratuito.
	if persisted, err := h.streams.FindByChannelID(r.Context(), id); err == nil && len(persisted) > 0 {
		return persisted[0].URL, nil
	}

	// 3. La DB no sabe nada del canal: arranque en frío antes del primer sync.
	return h.provider.GetStreamURL(r.Context(), id)
}

// GetHealth resume el estado de salud de un canal a partir de sus streams.
// Ruta: GET /channels/{id}/health → {alive, latency_ms, checked_at}
func (h *ChannelHandler) GetHealth(w http.ResponseWriter, r *http.Request) {
	id := pathParam(r, "id")
	if id == "" {
		h.writeError(w, http.StatusBadRequest, "id requerido")
		return
	}

	streams, err := h.streams.FindByChannelID(r.Context(), domain.ChannelID(id))
	if err != nil {
		h.logger.Error("GetHealth: fallo consultando streams",
			slog.String("channel", id), slog.Any("error", err))
		h.writeError(w, http.StatusInternalServerError, "Error obteniendo salud del canal")
		return
	}
	if len(streams) == 0 {
		h.writeError(w, http.StatusNotFound, "Canal sin streams")
		return
	}

	// alive si algún stream lo está; latencia = la mejor entre los vivos;
	// checked_at = el chequeo más reciente de cualquiera de sus streams.
	var (
		alive       bool
		bestLatency int64
		lastChecked time.Time
	)
	for _, s := range streams {
		if s.IsAlive && (!alive || s.LatencyMs < bestLatency) {
			alive = true
			bestLatency = s.LatencyMs
		}
		if s.LastChecked.After(lastChecked) {
			lastChecked = s.LastChecked
		}
	}

	res := map[string]any{
		"alive":      alive,
		"latency_ms": bestLatency,
	}
	if lastChecked.IsZero() {
		res["checked_at"] = nil // aún sin pasada del health-worker
	} else {
		res["checked_at"] = lastChecked.Format(time.RFC3339)
	}
	h.writeJSON(w, http.StatusOK, res)
}

// mirrorJSON es la forma de cable de /channels/streams. Endpoint nuevo, no lo
// consume Flutter: etiquetas limpias en minúsculas.
type mirrorJSON struct {
	URL       string `json:"url"`
	IsAlive   bool   `json:"is_alive"`
	LatencyMs int64  `json:"latency_ms"`
	WebOK     *bool  `json:"web_ok"`   // null = sin comprobar
	CodecOK   *bool  `json:"codec_ok"` // null = sin sondear; false = ningún navegador decodifica su vídeo
	// Codecs por el cable lleva SOLO el/los códec(s) de vídeo (p.ej. "mpeg2video"),
	// nunca la lista entera del PMT: la columna streams.codecs de la DB sí guarda
	// esa lista completa (con audio y tipos desconocidos) para el censo, pero el
	// mensaje al usuario nombra un formato de VÍDEO y decir «viene en aac» sería
	// mentir el códec equivocado. '' si no se sabe o si el PMT no traía vídeo.
	Codecs string `json:"codecs"`
}

// GetChannelStreams devuelve los mirrors de un canal ordenados por salud, para
// que el cliente haga failover. Ruta: GET /channels/streams?id=<channelID>
func (h *ChannelHandler) GetChannelStreams(w http.ResponseWriter, r *http.Request) {
	id := domain.ChannelID(r.URL.Query().Get("id"))
	if id == "" {
		h.writeError(w, http.StatusBadRequest, "id requerido")
		return
	}
	mirrors, err := h.streams.FindMirrorsByChannelID(r.Context(), id)
	if err != nil {
		h.logger.Error("GetChannelStreams: fallo buscando mirrors", slog.Any("error", err))
		h.writeError(w, http.StatusInternalServerError, "Error buscando mirrors")
		return
	}
	if len(mirrors) == 0 {
		h.writeError(w, http.StatusNotFound, "Canal sin mirrors")
		return
	}
	salida := make([]mirrorJSON, 0, len(mirrors))
	for _, m := range mirrors {
		salida = append(salida, mirrorJSON{
			URL: m.URL, IsAlive: m.IsAlive, LatencyMs: m.LatencyMs, WebOK: webOKaPtr(m.WebOK),
			CodecOK: codecOKaPtr(m.Codec), Codecs: domain.CodecsDeVideo(m.Codecs),
		})
	}
	h.writeJSON(w, http.StatusOK, salida)
}

// webOKaPtr traduce el tri-estado al *bool del cable (nil = sin comprobar).
func webOKaPtr(v domain.WebSupport) *bool {
	switch v {
	case domain.WebOK:
		t := true
		return &t
	case domain.WebNo:
		f := false
		return &f
	default:
		return nil
	}
}

// codecOKaPtr traduce el tri-estado del sondeo de códecs al *bool del cable
// (nil = sin sondear).
func codecOKaPtr(v domain.CodecSupport) *bool {
	switch v {
	case domain.CodecOK:
		t := true
		return &t
	case domain.CodecNo:
		f := false
		return &f
	default:
		return nil
	}
}

// GetCountries y GetCategories alimentan los selectores de filtro de la app:
// cada valor con su recuento real, para que el usuario vea cuánto hay detrás de
// cada opción antes de elegirla.
func (h *ChannelHandler) GetCountries(w http.ResponseWriter, r *http.Request) {
	facetas, err := h.repo.Countries(r.Context())
	if err != nil {
		h.logger.Error("GetCountries: fallo consultando países", slog.Any("error", err))
		h.writeError(w, http.StatusInternalServerError, "Error obteniendo países")
		return
	}
	h.writeJSON(w, http.StatusOK, facetas)
}

func (h *ChannelHandler) GetCategories(w http.ResponseWriter, r *http.Request) {
	facetas, err := h.repo.Categories(r.Context())
	if err != nil {
		h.logger.Error("GetCategories: fallo consultando categorías", slog.Any("error", err))
		h.writeError(w, http.StatusInternalServerError, "Error obteniendo categorías")
		return
	}
	h.writeJSON(w, http.StatusOK, facetas)
}

// GetQualities alimenta la faceta de calidad del cliente web (hd/fhd/4k) con
// el recuento real de cada tramo, reutilizando el mismo predicado de
// resolución que el filtro quality= de /channels. Solo lo consume el
// cliente web; Flutter no lo conoce.
func (h *ChannelHandler) GetQualities(w http.ResponseWriter, r *http.Request) {
	facetas, err := h.repo.Qualities(r.Context())
	if err != nil {
		h.logger.Error("GetQualities: fallo consultando calidades", slog.Any("error", err))
		h.writeError(w, http.StatusInternalServerError, "Error obteniendo calidades")
		return
	}
	h.writeJSON(w, http.StatusOK, facetas)
}

// GetRandom devuelve un canal al azar entre los que casan con el filtro. Se
// fuerza AliveOnly: un aleatorio muerto arruina la función.
// Ruta: GET /channels/random?country=&category=&quality=
func (h *ChannelHandler) GetRandom(w http.ResponseWriter, r *http.Request) {
	filtro := filtroDesdeQuery(r)
	quality := r.URL.Query().Get("quality")
	if !strings.EqualFold(quality, "all") {
		filtro.MinQuality = strings.ToLower(quality)
	}
	filtro.AliveOnly = true

	canal, err := h.repo.Random(r.Context(), filtro)
	if err != nil {
		// Sin coincidencias no es un fallo del servidor: el usuario ha filtrado
		// hasta dejar el conjunto vacío.
		h.writeError(w, http.StatusNotFound, "Ningún canal casa con el filtro")
		return
	}
	h.writeJSON(w, http.StatusOK, canal)
}

// filtroDesdeQuery lee los parámetros comunes a /channels y /channels/random,
// en un solo sitio para que los dos no puedan divergir.
func filtroDesdeQuery(r *http.Request) ports.ChannelFilter {
	f := ports.ChannelFilter{
		Query:    r.URL.Query().Get("q"),
		Country:  r.URL.Query().Get("country"),
		Category: r.URL.Query().Get("category"),
	}
	if ids := r.URL.Query().Get("ids"); ids != "" {
		for _, id := range strings.Split(ids, ",") {
			if id = strings.TrimSpace(id); id != "" {
				f.IDs = append(f.IDs, id)
			}
		}
	}
	return f
}

func queryInt(r *http.Request, key string, def, max int) int {
	v, err := strconv.Atoi(r.URL.Query().Get(key))
	if err != nil || v < 0 {
		return def
	}
	if max > 0 && v > max {
		return max
	}
	return v
}

func (h *ChannelHandler) writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (h *ChannelHandler) writeError(w http.ResponseWriter, status int, msg string) {
	h.writeJSON(w, status, map[string]string{"error": msg})
}
