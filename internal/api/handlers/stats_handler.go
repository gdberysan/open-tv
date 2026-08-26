package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/gdberysan/open-tv/internal/stats"
)

// maxCuerpoPlayback acota el body de POST /stats/playback: un evento de
// reproducción no tiene por qué pesar más que un par de campos cortos. Sin
// límite, cualquiera con acceso a loopback podría inflar la memoria del
// proceso a base de "motivo" gigantes.
const maxCuerpoPlayback = 4 << 10

// CatalogoStats expone el agregado del catálogo (streams vivos/muertos/web_ok)
// que GET /stats compone junto al agregado de reproducción. Es una interfaz
// mínima y no el SyncStatus de /health: /health no calcula ese agregado (solo
// la edad del último sync), así que en vez de forzar algo ahí encima se
// define esta interfaz nueva, mínima a propósito, para que el handler no se
// acople a una fuente concreta — en producción la satisface una consulta
// directa a la DB de solo lectura (DBCatalogoStats); los tests pueden pasar
// nil (sin agregado de catálogo) o un stub.
type CatalogoStats interface {
	Resumen(ctx context.Context) (map[string]any, error)
}

// DBCatalogoStats calcula el agregado de streams directamente contra la DB de
// solo lectura. No existe un query de este tipo en ningún repositorio: los
// repos exponen streams por canal, no un agregado global del catálogo entero.
// En vez de ensanchar ports.StreamRepository con un método que solo usa este
// endpoint, se hace la única consulta SQL aquí: es lo más simple y no toca el
// contrato de los repos existentes.
type DBCatalogoStats struct {
	db *sql.DB
}

func NewDBCatalogoStats(db *sql.DB) *DBCatalogoStats {
	return &DBCatalogoStats{db: db}
}

func (c *DBCatalogoStats) Resumen(ctx context.Context) (map[string]any, error) {
	const q = `SELECT
		COUNT(*),
		COALESCE(SUM(is_alive), 0),
		COALESCE(SUM(CASE WHEN web_ok = 1 THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN web_ok = 0 THEN 1 ELSE 0 END), 0)
		FROM streams`
	var total, vivos, webOK, webNo int
	if err := c.db.QueryRowContext(ctx, q).Scan(&total, &vivos, &webOK, &webNo); err != nil {
		return nil, fmt.Errorf("agregando el catálogo: %w", err)
	}
	return map[string]any{
		"streams_totales": total,
		"vivos":           vivos,
		"muertos":         total - vivos,
		"web_ok":          webOK,
		"web_no":          webNo,
		"web_desconocido": total - webOK - webNo,
	}, nil
}

// StatsHandler responde los endpoints de observabilidad local: el cliente
// reporta desenlaces de reproducción y este handler los agrega en memoria
// (nunca a disco, nunca fuera de la máquina) y los expone junto al agregado
// del catálogo.
type StatsHandler struct {
	agg      *stats.Agregador
	catalogo CatalogoStats
}

func NewStatsHandler(agg *stats.Agregador, catalogo CatalogoStats) *StatsHandler {
	if agg == nil {
		agg = stats.NuevoAgregador()
	}
	return &StatsHandler{agg: agg, catalogo: catalogo}
}

// playbackBody es el contrato con el cliente web (Tarea 14): canal_id y
// resultado son obligatorios, el resto describe el desenlace y es opcional.
type playbackBody struct {
	CanalID       string `json:"canal_id"`
	Resultado     string `json:"resultado"`
	Motivo        string `json:"motivo"`
	Motor         string `json:"motor"`
	Via           string `json:"via"`
	MirrorIndex   int    `json:"mirror_index"`
	MsPrimerFrame int    `json:"ms_primer_frame"`
}

// PostPlayback registra un desenlace de reproducción. El body se lee acotado
// (Ruling: cero superficie de ataque gratis en un endpoint sin auth) y un
// cuerpo que se pasa del límite se rechaza con 400, igual que uno malformado.
func (h *StatsHandler) PostPlayback(w http.ResponseWriter, r *http.Request) {
	var body playbackBody
	dec := json.NewDecoder(io.LimitReader(r.Body, maxCuerpoPlayback))
	if err := dec.Decode(&body); err != nil {
		h.writeError(w, http.StatusBadRequest, "cuerpo invalido o demasiado grande")
		return
	}
	if body.CanalID == "" || body.Resultado == "" {
		h.writeError(w, http.StatusBadRequest, "canal_id y resultado son obligatorios")
		return
	}

	h.agg.Registrar(stats.Desenlace{
		CanalID:       body.CanalID,
		Resultado:     body.Resultado,
		Motivo:        body.Motivo,
		Motor:         body.Motor,
		Via:           body.Via,
		MirrorIndex:   body.MirrorIndex,
		MsPrimerFrame: body.MsPrimerFrame,
	})
	w.WriteHeader(http.StatusNoContent)
}

// GetStats compone el agregado de reproducción con el del catálogo. Un fallo
// del agregado de catálogo (p.ej. la DB no responde) no puede tumbar todo el
// endpoint: se omite esa clave y se registra el aviso, pero el de
// reproducción — que nunca toca disco — siempre se sirve.
func (h *StatsHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	resp := map[string]any{
		"reproduccion": h.agg.Resumen(),
	}
	if h.catalogo != nil {
		cat, err := h.catalogo.Resumen(r.Context())
		if err != nil {
			slog.Warn("GetStats: fallo agregando el catálogo", slog.Any("error", err))
		} else {
			resp["catalogo"] = cat
		}
	}
	h.writeJSON(w, http.StatusOK, resp)
}

func (h *StatsHandler) writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (h *StatsHandler) writeError(w http.ResponseWriter, status int, msg string) {
	h.writeJSON(w, status, map[string]string{"error": msg})
}
