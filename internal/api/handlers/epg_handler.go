package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gdberysan/open-tv/internal/domain"
	"github.com/gdberysan/open-tv/internal/ports"
)

// maxEPGIDs acota cuántos channelIDs acepta GET /channels/epg de una vez: como
// el resto de la API con ids= (ver filtroDesdeQuery), un lote sin tope es
// superficie de abuso gratis en un endpoint sin auth. Los ids de más allá del
// tope se descartan en silencio, igual que queryInt recorta limit/offset.
const maxEPGIDs = 1000

// defaultProximosLimit/maxProximosLimit acotan ?limit= de
// GET /channels/{id}/epg: "unos pocos próximos" (ver el diseño de P2), no el
// historial entero de la fuente.
const (
	defaultProximosLimit = 5
	maxProximosLimit     = 20
)

// EPGHandler sirve la guía EPG (ahora/después) al cliente, en dos formas: en
// lote para varios canales (la vista de catálogo) y detallada para uno solo
// (el overlay de un canal). Consume ports.EPGRepository — ni Podar ni
// ReemplazarVentana (esos son del Syncer, T3/T4), solo las dos consultas de
// lectura.
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

// programaJSON es la forma de cable (snake_case) de un domain.Programa: solo
// lo que el cliente necesita para pintar "ahora"/"siguiente" — título e
// inicio/fin en epoch UTC. Subtítulo y descripción no viajan (el diseño de P2
// no los pide en el catálogo ni en el overlay; ver docs/superpowers/specs).
type programaJSON struct {
	Titulo string `json:"titulo"`
	Inicio int64  `json:"inicio"`
	Fin    int64  `json:"fin"`
}

// programaAJSON traduce el puntero de dominio (nil = sin dato) al puntero de
// cable (nil = "null" explícito en el JSON, nunca un objeto con campos a
// blanco).
func programaAJSON(p *domain.Programa) *programaJSON {
	if p == nil {
		return nil
	}
	return &programaJSON{Titulo: p.Titulo, Inicio: p.InicioUTC, Fin: p.FinUTC}
}

// ahoraDespuesJSON es la forma de cable de domain.AhoraDespues.
type ahoraDespuesJSON struct {
	Ahora     *programaJSON `json:"ahora"`
	Siguiente *programaJSON `json:"siguiente"`
}

// GetEPGLote sirve ahora/después de varios canales a la vez.
// Ruta: GET /channels/epg?ids=a,b,c
//
// Un channelID cuyo canal no tiene tvg_id (sin guía posible en ninguna
// fuente) queda AUSENTE de la respuesta — ports.EPGRepository.
// AhoraDespuesPorCanales ya lo garantiza así, y este handler se limita a
// serializar el mapa que devuelve: no añade ni quita claves. `ids` vacío
// devuelve `{}` sin tocar el repositorio (no "todos los canales").
func (h *EPGHandler) GetEPGLote(w http.ResponseWriter, r *http.Request) {
	ids := parseIDsAcotado(r.URL.Query().Get("ids"), maxEPGIDs)
	if len(ids) == 0 {
		h.writeJSON(w, http.StatusOK, map[string]ahoraDespuesJSON{})
		return
	}

	mapa, err := h.repo.AhoraDespuesPorCanales(r.Context(), ids, time.Now().Unix())
	if err != nil {
		h.logger.Error("GetEPGLote: fallo consultando la guía", slog.Any("error", err))
		h.writeError(w, http.StatusInternalServerError, "Error obteniendo la guía")
		return
	}

	salida := make(map[string]ahoraDespuesJSON, len(mapa))
	for id, ad := range mapa {
		salida[id] = ahoraDespuesJSON{
			Ahora:     programaAJSON(ad.Ahora),
			Siguiente: programaAJSON(ad.Siguiente),
		}
	}
	h.writeJSON(w, http.StatusOK, salida)
}

// epgCanalJSON es la forma de cable de GET /channels/{id}/epg.
type epgCanalJSON struct {
	Ahora    *programaJSON  `json:"ahora"`
	Proximos []programaJSON `json:"proximos"`
}

// GetEPGCanal sirve ahora + los próximos programas de UN canal.
// Ruta: GET /channels/{id}/epg?limit=<n> (default 5, tope 20)
//
// ports.EPGRepository.ProximosDeCanal(limite=N) puede devolver hasta N+1
// filas cuando no hay programa en curso (un hueco): pide N+1 para poder
// distinguir "hay un programa cubriendo `ahora`" de "no lo hay" sin una
// segunda consulta, y dejar aun así N próximos completos en cualquiera de los
// dos casos. Este handler separa esa fila (si existe, como mucho una: el
// propio repo garantiza que start_utc<=ahora<stop_utc no puede cumplirlo dos
// programas de la misma fuente a la vez) del resto, y RECORTA `proximos` a
// como mucho N — necesario en el caso sin hueco, donde las N+1 filas
// devueltas son todas futuras.
func (h *EPGHandler) GetEPGCanal(w http.ResponseWriter, r *http.Request) {
	id := pathParam(r, "id")
	if id == "" {
		h.writeError(w, http.StatusBadRequest, "id requerido")
		return
	}
	limite := queryInt(r, "limit", defaultProximosLimit, maxProximosLimit)
	ahora := time.Now().Unix()

	programas, err := h.repo.ProximosDeCanal(r.Context(), id, ahora, limite)
	if err != nil {
		h.logger.Error("GetEPGCanal: fallo consultando la guía",
			slog.String("canal", id), slog.Any("error", err))
		h.writeError(w, http.StatusInternalServerError, "Error obteniendo la guía")
		return
	}

	var actual *domain.Programa
	proximos := make([]programaJSON, 0, limite)
	for i := range programas {
		p := programas[i]
		if actual == nil && p.InicioUTC <= ahora && ahora < p.FinUTC {
			actual = &p
			continue
		}
		if len(proximos) >= limite {
			continue // el +1 de sobra del repo cuando no hubo "ahora" que descartar
		}
		proximos = append(proximos, *programaAJSON(&p))
	}

	h.writeJSON(w, http.StatusOK, epgCanalJSON{
		Ahora:    programaAJSON(actual),
		Proximos: proximos,
	})
}

// parseIDsAcotado divide una lista "a,b,c" (recortando espacios y descartando
// vacíos, igual que filtroDesdeQuery) y se detiene en cuanto alcanza max: el
// resto del querystring se ignora en silencio, no es un error.
func parseIDsAcotado(raw string, max int) []string {
	if raw == "" {
		return nil
	}
	var out []string
	for _, id := range strings.Split(raw, ",") {
		if id = strings.TrimSpace(id); id != "" {
			out = append(out, id)
			if len(out) >= max {
				break
			}
		}
	}
	return out
}

func (h *EPGHandler) writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (h *EPGHandler) writeError(w http.ResponseWriter, status int, msg string) {
	h.writeJSON(w, status, map[string]string{"error": msg})
}
