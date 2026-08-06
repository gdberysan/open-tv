package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/tu-org/iptv-ecosystem/gateway/internal/domain"
	"github.com/tu-org/iptv-ecosystem/gateway/internal/ports"
)

type ChannelHandler struct {
	repo     ports.ChannelRepository
	provider ports.ProviderPort
}

func NewChannelHandler(repo ports.ChannelRepository, provider ports.ProviderPort) *ChannelHandler {
	return &ChannelHandler{repo: repo, provider: provider}
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

	channels, err := h.repo.FindFiltered(r.Context(), ports.ChannelFilter{
		Query:      r.URL.Query().Get("q"),
		Country:    r.URL.Query().Get("country"),
		Category:   r.URL.Query().Get("category"),
		MinQuality: quality,
		Limit:      queryInt(r, "limit", 500, 1000),
		Offset:     queryInt(r, "offset", 0, -1),
	})
	if err != nil {
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
		h.writeError(w, http.StatusNotFound, "Stream no encontrado")
		return
	}
	h.writeJSON(w, http.StatusOK, map[string]string{"url": url})
}

// GetEPG devuelve la guía de programación para un canal.
func (h *ChannelHandler) GetEPG(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		h.writeError(w, http.StatusBadRequest, "id requerido")
		return
	}
	epg, err := h.provider.GetEPGData(r.Context(), domain.ChannelID(id))
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "Error obteniendo EPG")
		return
	}
	h.writeJSON(w, http.StatusOK, epg)
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
