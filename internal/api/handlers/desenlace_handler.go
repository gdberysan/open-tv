package handlers

import (
	"encoding/json"
	"io"
	"log/slog"
	"math"
	"net/http"

	"github.com/gdberysan/open-tv/internal/ports"
)

// DesenlaceHandler recibe del reproductor el desenlace REAL de un intento
// sobre un mirror (spec tiempo-hasta-la-imagen §3.1/§3.5). Mismo origen
// (MismoOrigen es global), best-effort para el cliente, escritura por el
// pool de escritura que main.go inyecta.
type DesenlaceHandler struct {
	logger      *slog.Logger
	registrador ports.RegistradorDesenlaces
}

func NewDesenlaceHandler(logger *slog.Logger, registrador ports.RegistradorDesenlaces) *DesenlaceHandler {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	return &DesenlaceHandler{logger: logger, registrador: registrador}
}

type desenlaceBody struct {
	URL           string  `json:"url"`
	Resultado     string  `json:"resultado"`
	Motivo        string  `json:"motivo"`
	MsPrimerFrame float64 `json:"ms_primer_frame"` // float: performance.now() trae decimales
}

// Post registra POST /streams/desenlace. Sin registrador (nil, el caso de
// los tests de router que no lo inyectan) responde 503 en vez de panicar: el
// endpoint es best-effort para el cliente, nunca debe tumbar el proceso.
func (h *DesenlaceHandler) Post(w http.ResponseWriter, r *http.Request) {
	if h.registrador == nil {
		http.Error(w, "registro de desenlaces no disponible", http.StatusServiceUnavailable)
		return
	}
	var body desenlaceBody
	if err := json.NewDecoder(io.LimitReader(r.Body, maxCuerpoPlayback)).Decode(&body); err != nil {
		http.Error(w, "cuerpo invalido o demasiado grande", http.StatusBadRequest)
		return
	}
	if body.URL == "" || (body.Resultado != "iniciado" && body.Resultado != "fallo") {
		http.Error(w, "url y resultado (iniciado|fallo) son obligatorios", http.StatusBadRequest)
		return
	}
	// Se acota ANTES de convertir a int64: un float fuera de rango (o
	// negativo) convertido a int64 es comportamiento indefinido en Go.
	// 600000 ms (10 min) es más que cualquier arranque real.
	msPrimerFrame := body.MsPrimerFrame
	switch {
	case msPrimerFrame < 0:
		msPrimerFrame = 0
	case msPrimerFrame > 600000:
		msPrimerFrame = 600000
	}
	d := ports.DesenlaceMirror{Resultado: body.Resultado, Motivo: body.Motivo, MsPrimerFrame: int64(math.Round(msPrimerFrame))}
	if err := h.registrador.RegistrarDesenlace(r.Context(), body.URL, d); err != nil {
		h.logger.Warn("desenlace: fallo registrando", slog.Any("error", err))
		http.Error(w, "no se pudo registrar", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
