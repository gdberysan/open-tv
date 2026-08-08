package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/tu-org/iptv-ecosystem/gateway/internal/domain"
	"github.com/tu-org/iptv-ecosystem/gateway/internal/ports"
)

// EPGHandler sirve la guía de programación desde la DB local.
type EPGHandler struct {
	logger *slog.Logger
	repo   ports.EPGRepository
}

func NewEPGHandler(logger *slog.Logger, repo ports.EPGRepository) *EPGHandler {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	return &EPGHandler{logger: logger, repo: repo}
}

const (
	defaultEPGWindow = 6 * time.Hour
	maxEPGWindow     = 24 * time.Hour
)

// GetWindow devuelve la programación de un canal en una ventana temporal.
// Ruta: GET /epg?channel=<channelID>&from=<RFC3339>&to=<RFC3339>
// Defaults: from=ahora, to=from+6h. Ventana máxima: 24h.
func (h *EPGHandler) GetWindow(w http.ResponseWriter, r *http.Request) {
	channel := r.URL.Query().Get("channel")
	if channel == "" {
		h.writeError(w, http.StatusBadRequest, "channel requerido")
		return
	}

	from := time.Now()
	if v := r.URL.Query().Get("from"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			h.writeError(w, http.StatusBadRequest, "from inválido (RFC3339)")
			return
		}
		from = t
	}

	to := from.Add(defaultEPGWindow)
	if v := r.URL.Query().Get("to"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			h.writeError(w, http.StatusBadRequest, "to inválido (RFC3339)")
			return
		}
		to = t
	}

	if !to.After(from) {
		h.writeError(w, http.StatusBadRequest, "to debe ser posterior a from")
		return
	}
	if to.Sub(from) > maxEPGWindow {
		h.writeError(w, http.StatusBadRequest, "ventana máxima: 24h")
		return
	}

	entries, err := h.repo.FindByChannelAndWindow(r.Context(), domain.ChannelID(channel), from, to)
	if err != nil {
		h.logger.Error("EPG: fallo consultando la guía", slog.Any("error", err))
		h.writeError(w, http.StatusInternalServerError, "Error obteniendo EPG")
		return
	}
	h.writeEntries(w, entries)
}

// GetByChannel es la ruta legacy /channels/{id}/epg: próximas 6 horas del
// canal. Nota: los IDs con "/" no funcionan como path param; para esos casos
// existe GET /epg?channel=.
func (h *EPGHandler) GetByChannel(w http.ResponseWriter, r *http.Request) {
	id := pathParam(r, "id")
	if id == "" {
		h.writeError(w, http.StatusBadRequest, "id requerido")
		return
	}
	now := time.Now()
	entries, err := h.repo.FindByChannelAndWindow(r.Context(), domain.ChannelID(id), now, now.Add(defaultEPGWindow))
	if err != nil {
		h.logger.Error("EPG: fallo consultando la guía", slog.Any("error", err))
		h.writeError(w, http.StatusInternalServerError, "Error obteniendo EPG")
		return
	}
	h.writeEntries(w, entries)
}

// GetNow devuelve el programa en emisión de todos los canales con EPG.
// Ruta: GET /epg/now
func (h *EPGHandler) GetNow(w http.ResponseWriter, r *http.Request) {
	entries, err := h.repo.FindCurrentlyAiring(r.Context(), time.Now())
	if err != nil {
		h.logger.Error("EPG: fallo consultando la guía", slog.Any("error", err))
		h.writeError(w, http.StatusInternalServerError, "Error obteniendo EPG")
		return
	}
	h.writeEntries(w, entries)
}

func (h *EPGHandler) writeEntries(w http.ResponseWriter, entries []domain.EPGEntry) {
	if entries == nil {
		// `[]`, no `null`: los clientes JSON no deben lidiar con null
		entries = []domain.EPGEntry{}
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(entries); err != nil {
		slog.Error("Error serializando EPG", slog.Any("error", err))
	}
}

func (h *EPGHandler) writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
