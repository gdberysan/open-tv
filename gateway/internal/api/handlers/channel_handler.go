package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/tu-org/iptv-ecosystem/gateway/internal/domain"
	"github.com/tu-org/iptv-ecosystem/gateway/internal/ports"
)

type ChannelHandler struct {
	logger   *slog.Logger
	repo     ports.ChannelRepository
	provider ports.ProviderPort
	streams  ports.StreamRepository
}

func NewChannelHandler(logger *slog.Logger, repo ports.ChannelRepository, provider ports.ProviderPort, streams ports.StreamRepository) *ChannelHandler {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	return &ChannelHandler{logger: logger, repo: repo, provider: provider, streams: streams}
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

	channels, err := h.repo.FindFiltered(r.Context(), ports.ChannelFilter{
		Query:      r.URL.Query().Get("q"),
		Country:    r.URL.Query().Get("country"),
		Category:   r.URL.Query().Get("category"),
		MinQuality: quality,
		AliveOnly:  aliveOnly,
		Limit:      queryInt(r, "limit", 500, 1000),
		Offset:     queryInt(r, "offset", 0, -1),
	})
	if err != nil {
		h.logger.Error("GetChannels: fallo consultando el catálogo", slog.Any("error", err))
		h.writeError(w, http.StatusInternalServerError, "Error obteniendo canales")
		return
	}
	h.writeJSON(w, http.StatusOK, channels)
}

// GetStreamURL resuelve la URL de reproducción para un canal.
// Ruta: GET /channels/stream?id=<channelID>
// Usa query param para soportar IDs con "/" (ej: "24/7 News").
func (h *ChannelHandler) GetStreamURL(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		h.writeError(w, http.StatusBadRequest, "id requerido")
		return
	}
	url, err := h.provider.GetStreamURL(r.Context(), domain.ChannelID(id))
	if err != nil {
		// Caché del provider vacío (p.ej. tras un reinicio, antes del primer
		// sync): caer a los streams persistidos en DB por el último sync.
		persisted, dbErr := h.streams.FindByChannelID(r.Context(), domain.ChannelID(id))
		if dbErr != nil || len(persisted) == 0 {
			h.writeError(w, http.StatusNotFound, "Stream no encontrado")
			return
		}
		url = persisted[0].URL
	}
	h.writeJSON(w, http.StatusOK, map[string]string{"url": url})
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
