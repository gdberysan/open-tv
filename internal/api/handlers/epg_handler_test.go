package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gdberysan/open-tv/internal/domain"
	"github.com/gdberysan/open-tv/internal/ports"
	"github.com/go-chi/chi/v5"
)

// fakeEPGRepo es un doble de ports.EPGRepository. Las dos consultas de
// escritura (ReemplazarVentana/Podar) son del Syncer, no de este handler: se
// dejan como no-op. proximosFn recibe el `ahora` REAL que calcula el handler
// (time.Now().Unix()), así el test construye programas relativos a ese
// instante en vez de tener que adivinarlo o inyectar un reloj.
type fakeEPGRepo struct {
	mapa    map[string]domain.AhoraDespues
	mapaErr error
	lastIDs []string

	proximosFn    func(ahora int64, limite int) []domain.Programa
	proximosErr   error
	lastChannelID string
	lastLimite    int
}

var _ ports.EPGRepository = (*fakeEPGRepo)(nil)

func (f *fakeEPGRepo) ReemplazarVentana(context.Context, string, []domain.Programa) error {
	return nil
}
func (f *fakeEPGRepo) Podar(context.Context, int64) error { return nil }

func (f *fakeEPGRepo) AhoraDespuesPorCanales(_ context.Context, channelIDs []string, _ int64) (map[string]domain.AhoraDespues, error) {
	f.lastIDs = channelIDs
	if f.mapaErr != nil {
		return nil, f.mapaErr
	}
	return f.mapa, nil
}

func (f *fakeEPGRepo) ProximosDeCanal(_ context.Context, channelID string, ahora int64, limite int) ([]domain.Programa, error) {
	f.lastChannelID = channelID
	f.lastLimite = limite
	if f.proximosErr != nil {
		return nil, f.proximosErr
	}
	if f.proximosFn == nil {
		return nil, nil
	}
	return f.proximosFn(ahora, limite), nil
}

// programaCable es la forma de cable que se decodifica en los tests, reflejo
// exacto de programaJSON (no se reutiliza el tipo no exportado a propósito:
// un test que decodifica desde el JSON real, no desde la struct interna,
// prueba el contrato de cable de verdad).
type programaCable struct {
	Titulo string `json:"titulo"`
	Inicio int64  `json:"inicio"`
	Fin    int64  `json:"fin"`
}

type ahoraDespuesCable struct {
	Ahora     *programaCable `json:"ahora"`
	Siguiente *programaCable `json:"siguiente"`
}

func TestEPGHandler_GetEPGLote_FormaEsperada(t *testing.T) {
	repo := &fakeEPGRepo{mapa: map[string]domain.AhoraDespues{
		"a": {
			Ahora:     &domain.Programa{Titulo: "Noticias", InicioUTC: 1000, FinUTC: 2000},
			Siguiente: &domain.Programa{Titulo: "Deportes", InicioUTC: 2000, FinUTC: 3000},
		},
		"b": {}, // tiene guía posible (tvg_id) pero nada cubre ahora ni hay siguiente
	}}
	h := NewEPGHandler(nil, repo)

	req := httptest.NewRequest(http.MethodGet, "/channels/epg?ids=a,b", nil)
	rr := httptest.NewRecorder()
	h.GetEPGLote(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rr.Code, rr.Body.String())
	}
	var out map[string]ahoraDespuesCable
	if err := json.NewDecoder(rr.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v (body %s)", err, rr.Body.String())
	}
	if len(out) != 2 {
		t.Fatalf("claves = %d, quiero 2 (a,b); out=%+v", len(out), out)
	}

	a, ok := out["a"]
	if !ok {
		t.Fatal("falta la clave 'a'")
	}
	if a.Ahora == nil || a.Ahora.Titulo != "Noticias" || a.Ahora.Inicio != 1000 || a.Ahora.Fin != 2000 {
		t.Errorf("a.ahora = %+v, quiero Noticias/1000/2000", a.Ahora)
	}
	if a.Siguiente == nil || a.Siguiente.Titulo != "Deportes" {
		t.Errorf("a.siguiente = %+v, quiero Deportes", a.Siguiente)
	}

	b, ok := out["b"]
	if !ok {
		t.Fatal("falta la clave 'b' (tiene guía, pero nada ahora): debe seguir presente con null")
	}
	if b.Ahora != nil || b.Siguiente != nil {
		t.Errorf("b = %+v, quiero ahora y siguiente null explícitos", b)
	}

	if len(repo.lastIDs) != 2 || repo.lastIDs[0] != "a" || repo.lastIDs[1] != "b" {
		t.Errorf("repo.lastIDs = %v, quiero [a b]", repo.lastIDs)
	}
}

func TestEPGHandler_GetEPGLote_CanalSinTvgIDQuedaAusente(t *testing.T) {
	// "c" se pide pero el repo (fiel a ports.EPGRepository) no lo incluye en
	// el mapa: sin tvg_id, sin guía posible. Debe quedar AUSENTE de la
	// respuesta, no aparecer con null — así el cliente distingue "sin guía"
	// de "hay guía pero nada ahora".
	repo := &fakeEPGRepo{mapa: map[string]domain.AhoraDespues{
		"a": {},
	}}
	h := NewEPGHandler(nil, repo)

	req := httptest.NewRequest(http.MethodGet, "/channels/epg?ids=a,c", nil)
	rr := httptest.NewRecorder()
	h.GetEPGLote(rr, req)

	var out map[string]json.RawMessage
	if err := json.NewDecoder(rr.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := out["c"]; ok {
		t.Errorf("'c' presente en la respuesta (%s); quiero ausente", out["c"])
	}
	if _, ok := out["a"]; !ok {
		t.Error("falta 'a'")
	}
}

func TestEPGHandler_GetEPGLote_IDsVacioDevuelveObjetoVacio(t *testing.T) {
	// El repo tiene datos, pero ids vacío no debe traducirse en "todos los
	// canales": debe devolver {} sin siquiera consultar el repositorio.
	repo := &fakeEPGRepo{mapa: map[string]domain.AhoraDespues{"deberia-no-verse": {}}}
	h := NewEPGHandler(nil, repo)

	req := httptest.NewRequest(http.MethodGet, "/channels/epg", nil)
	rr := httptest.NewRecorder()
	h.GetEPGLote(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	var out map[string]json.RawMessage
	if err := json.NewDecoder(rr.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v (body %s)", err, rr.Body.String())
	}
	if len(out) != 0 {
		t.Errorf("out = %+v, quiero {} (objeto vacío, no todos los canales)", out)
	}
	if repo.lastIDs != nil {
		t.Errorf("repo.lastIDs = %v, el repo no debería haberse llamado con ids vacío", repo.lastIDs)
	}
}

func TestEPGHandler_GetEPGLote_RespetaElTopeDeIDs(t *testing.T) {
	repo := &fakeEPGRepo{mapa: map[string]domain.AhoraDespues{}}
	h := NewEPGHandler(nil, repo)

	ids := make([]string, 0, maxEPGIDs+50)
	for i := 0; i < maxEPGIDs+50; i++ {
		ids = append(ids, fmt.Sprintf("c%d", i))
	}
	req := httptest.NewRequest(http.MethodGet, "/channels/epg?ids="+strings.Join(ids, ","), nil)
	rr := httptest.NewRecorder()
	h.GetEPGLote(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rr.Code, rr.Body.String())
	}
	if len(repo.lastIDs) != maxEPGIDs {
		t.Fatalf("repo.lastIDs = %d, quiero el tope %d", len(repo.lastIDs), maxEPGIDs)
	}
}

func TestEPGHandler_GetEPGLote_ErrorDelRepoEs500(t *testing.T) {
	repo := &fakeEPGRepo{mapaErr: errors.New("boom")}
	h := NewEPGHandler(nil, repo)

	req := httptest.NewRequest(http.MethodGet, "/channels/epg?ids=a", nil)
	rr := httptest.NewRecorder()
	h.GetEPGLote(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, quiero 500", rr.Code)
	}
}

// routerEPGCanal monta solo la ruta con {id}, igual que setupRouterFull en
// channel_handler_test.go: así pathParam(r, "id") recibe un valor real de
// chi, no uno inyectado a mano en el contexto.
func routerEPGCanal(h *EPGHandler) http.Handler {
	r := chi.NewRouter()
	r.Get("/channels/{id}/epg", h.GetEPGCanal)
	return r
}

func TestEPGHandler_GetEPGCanal_AhoraYProximos(t *testing.T) {
	repo := &fakeEPGRepo{
		proximosFn: func(ahora int64, _ int) []domain.Programa {
			return []domain.Programa{
				{Titulo: "En curso", InicioUTC: ahora - 100, FinUTC: ahora + 100},
				{Titulo: "Después 1", InicioUTC: ahora + 100, FinUTC: ahora + 200},
				{Titulo: "Después 2", InicioUTC: ahora + 200, FinUTC: ahora + 300},
			}
		},
	}
	h := NewEPGHandler(nil, repo)
	r := routerEPGCanal(h)

	req := httptest.NewRequest(http.MethodGet, "/channels/c1/epg?limit=5", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rr.Code, rr.Body.String())
	}
	var out struct {
		Ahora    *programaCable  `json:"ahora"`
		Proximos []programaCable `json:"proximos"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v (body %s)", err, rr.Body.String())
	}
	if out.Ahora == nil || out.Ahora.Titulo != "En curso" {
		t.Errorf("ahora = %+v, quiero 'En curso'", out.Ahora)
	}
	if len(out.Proximos) != 2 {
		t.Fatalf("proximos = %d, quiero 2 (sin contar 'ahora'); out=%+v", len(out.Proximos), out.Proximos)
	}
	if out.Proximos[0].Titulo != "Después 1" || out.Proximos[1].Titulo != "Después 2" {
		t.Errorf("proximos = %+v", out.Proximos)
	}
	if repo.lastChannelID != "c1" {
		t.Errorf("channelID pasado al repo = %q, quiero c1", repo.lastChannelID)
	}
	if repo.lastLimite != 5 {
		t.Errorf("limite pasado al repo = %d, quiero 5 (?limit=5)", repo.lastLimite)
	}
}

// TestEPGHandler_GetEPGCanal_HuecoRecortaAlLimite cubre el caso carreado por
// la review de T3: sin programa en curso, ports.EPGRepository.ProximosDeCanal
// puede devolver limite+1 filas (todas futuras). El handler tiene que
// recortar `proximos` a como mucho `limite`, no colar la fila de sobra.
func TestEPGHandler_GetEPGCanal_HuecoRecortaAlLimite(t *testing.T) {
	const limite = 3
	repo := &fakeEPGRepo{
		proximosFn: func(ahora int64, lim int) []domain.Programa {
			out := make([]domain.Programa, 0, lim+1)
			for i := 0; i < lim+1; i++ { // el repo real devuelve limite+1 en un hueco
				start := ahora + int64(100*(i+1))
				out = append(out, domain.Programa{
					Titulo:    fmt.Sprintf("P%d", i),
					InicioUTC: start,
					FinUTC:    start + 100,
				})
			}
			return out
		},
	}
	h := NewEPGHandler(nil, repo)
	r := routerEPGCanal(h)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/channels/c1/epg?limit=%d", limite), nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rr.Code, rr.Body.String())
	}
	var out struct {
		Ahora    *programaCable  `json:"ahora"`
		Proximos []programaCable `json:"proximos"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v (body %s)", err, rr.Body.String())
	}
	if out.Ahora != nil {
		t.Errorf("ahora = %+v, quiero null (puro hueco, nada en curso)", out.Ahora)
	}
	if len(out.Proximos) != limite {
		t.Fatalf("proximos = %d, quiero recortado a %d (el repo entregó %d)", len(out.Proximos), limite, limite+1)
	}
	if repo.lastLimite != limite {
		t.Errorf("limite pasado al repo = %d, quiero %d", repo.lastLimite, limite)
	}
}

func TestEPGHandler_GetEPGCanal_SinIDEs400(t *testing.T) {
	h := NewEPGHandler(nil, &fakeEPGRepo{})
	req := httptest.NewRequest(http.MethodGet, "/channels//epg", nil)
	rr := httptest.NewRecorder()
	h.GetEPGCanal(rr, req) // llamado directo: pathParam sin ruta montada devuelve ""

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, quiero 400", rr.Code)
	}
}

func TestEPGHandler_GetEPGCanal_ErrorDelRepoEs500(t *testing.T) {
	repo := &fakeEPGRepo{proximosErr: errors.New("boom")}
	h := NewEPGHandler(nil, repo)
	r := routerEPGCanal(h)

	req := httptest.NewRequest(http.MethodGet, "/channels/c1/epg", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, quiero 500", rr.Code)
	}
}
