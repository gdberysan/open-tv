package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"time"

	"github.com/gdberysan/open-tv/internal/domain"
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
		COALESCE(SUM(CASE WHEN web_ok = 0 THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN codec_ok = 0 THEN 1 ELSE 0 END), 0)
		FROM streams`
	var total, vivos, webOK, webNo, codecNo int
	if err := c.db.QueryRowContext(ctx, q).Scan(&total, &vivos, &webOK, &webNo, &codecNo); err != nil {
		return nil, fmt.Errorf("agregando el catálogo: %w", err)
	}

	imagenP50, err := c.imagenP50Ms(ctx)
	if err != nil {
		return nil, err
	}
	sinImagen, err := c.sinImagenCount(ctx)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"streams_totales": total,
		"vivos":           vivos,
		"muertos":         total - vivos,
		"web_ok":          webOK,
		"web_no":          webNo,
		"web_desconocido": total - webOK - webNo,
		"codec_no":        codecNo,
		"imagen_p50_ms":   imagenP50,
		"sin_imagen":      sinImagen,
	}, nil
}

// imagenP50Ms es la mediana de los tiempos hasta la imagen REALES (spec
// tiempo-hasta-la-imagen §3.5): elemento n/2 de la lista ascendente, 0 si no
// hay ninguno todavía. Se calcula en Go y no en SQL (SQLite no trae PERCENTILE
// nativo) sobre una columna que en catálogos reales cabe de sobra en memoria.
func (c *DBCatalogoStats) imagenP50Ms(ctx context.Context) (int64, error) {
	rows, err := c.db.QueryContext(ctx, `SELECT imagen_ms FROM streams WHERE imagen_ms > 0 ORDER BY imagen_ms`)
	if err != nil {
		return 0, fmt.Errorf("agregando imagen_p50_ms: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var valores []int64
	for rows.Next() {
		var ms int64
		if err := rows.Scan(&ms); err != nil {
			return 0, fmt.Errorf("agregando imagen_p50_ms: %w", err)
		}
		valores = append(valores, ms)
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("agregando imagen_p50_ms: %w", err)
	}
	if len(valores) == 0 {
		return 0, nil
	}
	return valores[len(valores)/2], nil
}

// sinImagenCount cuenta los mirrors que domain.SinImagen salta AHORA MISMO:
// el mismo umbral que aplica el servidor en /channels/streams, para que el
// número de este agregado y lo que ve el cliente no puedan divergir.
func (c *DBCatalogoStats) sinImagenCount(ctx context.Context) (int, error) {
	rows, err := c.db.QueryContext(ctx,
		`SELECT fallos_reales, ultimo_desenlace_at FROM streams WHERE fallos_reales >= ?`,
		domain.UmbralFallosReales)
	if err != nil {
		return 0, fmt.Errorf("agregando sin_imagen: %w", err)
	}
	defer func() { _ = rows.Close() }()

	ahora := time.Now()
	var n int
	for rows.Next() {
		var fallosReales int
		var ultimoUnix int64
		if err := rows.Scan(&fallosReales, &ultimoUnix); err != nil {
			return 0, fmt.Errorf("agregando sin_imagen: %w", err)
		}
		var ultimo time.Time
		if ultimoUnix > 0 {
			ultimo = time.Unix(ultimoUnix, 0)
		}
		if domain.SinImagen(fallosReales, ultimo, ahora) {
			n++
		}
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("agregando sin_imagen: %w", err)
	}
	return n, nil
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
	CanalID     string `json:"canal_id"`
	Resultado   string `json:"resultado"`
	Motivo      string `json:"motivo"`
	Motor       string `json:"motor"`
	Via         string `json:"via"`
	MirrorIndex int    `json:"mirror_index"`
	// float64, no int: el cliente mide con performance.now(), que trae
	// DECIMALES — un decode a int rechazaba 3128.5 con 400 y todos los
	// desenlaces 'iniciado' reales se perdían en silencio (bug cazado en el
	// gate en Chrome de reproductor-primero). Un número JSON es un número;
	// se redondea al registrar.
	MsPrimerFrame float64 `json:"ms_primer_frame"`
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
		MsPrimerFrame: int(math.Round(body.MsPrimerFrame)),
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
