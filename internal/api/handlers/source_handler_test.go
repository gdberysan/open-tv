package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gdberysan/open-tv/internal/ports"
	"github.com/go-chi/chi/v5"
)

// fakeSourceRepo reproduce, en memoria, el mismo esquema de id que
// db.SQLiteSourceRepository (fnv64a de la URL, prefijo "src-") y el mismo
// rechazo de duplicados (ErrFuenteDuplicada), para que los tests de este
// paquete ejerciten el mismo contrato que la implementación real sin arrastrar
// SQLite.
type fakeSourceRepo struct {
	mu      sync.Mutex
	fuentes map[string]ports.Source // clave: id
	// errAdd, si no es nil, hace que Add lo devuelva sin tocar el mapa —
	// simula un fallo de alta que NO es duplicado (DB ocupada, disco lleno)
	// para los tests de limpieza de huérfanos (H1).
	errAdd error
}

func newFakeSourceRepo() *fakeSourceRepo {
	return &fakeSourceRepo{fuentes: make(map[string]ports.Source)}
}

func fakeSourceID(url string) string {
	h := fnv.New64a()
	_, _ = h.Write([]byte(url))
	return fmt.Sprintf("src-%016x", h.Sum64())
}

func (f *fakeSourceRepo) List(context.Context) ([]ports.Source, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]ports.Source, 0, len(f.fuentes))
	for _, s := range f.fuentes {
		out = append(out, s)
	}
	return out, nil
}

func (f *fakeSourceRepo) Add(_ context.Context, s ports.Source) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.errAdd != nil {
		return f.errAdd
	}
	id := fakeSourceID(s.URL)
	if _, ok := f.fuentes[id]; ok {
		return fmt.Errorf("fake: %w", ports.ErrFuenteDuplicada)
	}
	s.ID = id
	f.fuentes[id] = s
	return nil
}

func (f *fakeSourceRepo) Remove(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.fuentes, id)
	return nil
}

func (f *fakeSourceRepo) TouchSync(_ context.Context, id string, cuando int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if s, ok := f.fuentes[id]; ok {
		s.UltimoSync = cuando
		f.fuentes[id] = s
	}
	return nil
}

// spySyncer registra cada llamada a SyncOne y avisa por un canal, para que
// los tests puedan esperar de forma determinista al goroutine asíncrono del
// handler en vez de sondear con sleeps.
type spySyncer struct {
	mu       sync.Mutex
	llamadas []string
	avisos   chan string
	err      error
}

func newSpySyncer() *spySyncer {
	return &spySyncer{avisos: make(chan string, 16)}
}

func (s *spySyncer) SyncOne(_ context.Context, id string) error {
	s.mu.Lock()
	s.llamadas = append(s.llamadas, id)
	s.mu.Unlock()
	s.avisos <- id
	return s.err
}

func setupSourceRouter(repo ports.SourceRepository, syncer sincronizadorPuntual, fuentesDir string) http.Handler {
	h := NewSourceHandler(slog.New(slog.DiscardHandler), repo, syncer, fuentesDir)
	r := chi.NewRouter()
	r.Route("/sources", func(r chi.Router) {
		r.Get("/", h.Get)
		r.Post("/", h.Post)
		r.Get("/sugeridas", h.Sugeridas)
		r.Delete("/{id}", h.Delete)
		r.Post("/{id}/sync", h.Sync)
	})
	return r
}

// esperarAviso falla el test si el spy no recibe una llamada a tiempo: acota
// la espera del goroutine asíncrono en vez de colgar la suite si algo rompe.
func esperarAviso(t *testing.T, avisos <-chan string) string {
	t.Helper()
	select {
	case id := <-avisos:
		return id
	case <-time.After(2 * time.Second):
		t.Fatal("el sync asíncrono no se disparó a tiempo")
		return ""
	}
}

func TestSourceHandler_GetListaVacia(t *testing.T) {
	r := setupSourceRouter(newFakeSourceRepo(), newSpySyncer(), t.TempDir())

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/sources", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, quiero 200", rec.Code)
	}
	var got []fuenteJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("respuesta no es JSON: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("lista = %v, quiero vacía", got)
	}
}

func TestSourceHandler_GetFormaSnakeCase(t *testing.T) {
	repo := newFakeSourceRepo()
	if err := repo.Add(context.Background(), ports.Source{URL: "https://ej.test/a.m3u", Label: "A", Kind: "url", IsActive: true}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	r := setupSourceRouter(repo, newSpySyncer(), t.TempDir())

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/sources", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, quiero 200", rec.Code)
	}
	var got []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("respuesta no es JSON: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("lista = %v, quiero 1 elemento", got)
	}
	for _, clave := range []string{"id", "label", "url", "kind", "ultimo_sync", "canales"} {
		if _, ok := got[0][clave]; !ok {
			t.Errorf("falta la clave %q en %v", clave, got[0])
		}
	}
	if got[0]["ultimo_sync"].(float64) != 0 {
		t.Errorf("ultimo_sync = %v, quiero 0 (nunca sincronizada)", got[0]["ultimo_sync"])
	}
}

func TestSourceHandler_PostJSONValido(t *testing.T) {
	syncer := newSpySyncer()
	r := setupSourceRouter(newFakeSourceRepo(), syncer, t.TempDir())

	body := `{"url":"https://ej.test/canales.m3u","label":"Mi lista"}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/sources", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, quiero 201 (body=%s)", rec.Code, rec.Body.String())
	}
	var got fuenteJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("respuesta no es JSON: %v", err)
	}
	if got.URL != "https://ej.test/canales.m3u" || got.Label != "Mi lista" || got.Kind != "url" {
		t.Errorf("fuente creada = %+v", got)
	}
	if got.ID == "" {
		t.Error("id vacío en la fuente creada")
	}

	// El sync inicial se dispara en segundo plano, sin bloquear la respuesta.
	id := esperarAviso(t, syncer.avisos)
	if id != got.ID {
		t.Errorf("SyncOne llamado con id=%q, quiero %q", id, got.ID)
	}
}

func TestSourceHandler_PostSinLabelUsaURLComoLabel(t *testing.T) {
	r := setupSourceRouter(newFakeSourceRepo(), newSpySyncer(), t.TempDir())

	body := `{"url":"https://ej.test/x.m3u"}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/sources", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, quiero 201", rec.Code)
	}
	var got fuenteJSON
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if got.Label != "https://ej.test/x.m3u" {
		t.Errorf("label = %q, quiero la URL", got.Label)
	}
}

func TestSourceHandler_PostEsquemaInvalido(t *testing.T) {
	casos := []string{
		`{"url":"ftp://ej.test/a.m3u"}`,
		`{"url":"file:///etc/passwd"}`,
		`{"url":""}`,
		`{"url":"no-es-una-url"}`,
	}
	for _, body := range casos {
		r := setupSourceRouter(newFakeSourceRepo(), newSpySyncer(), t.TempDir())
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/sources", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("body=%s: status = %d, quiero 400", body, rec.Code)
		}
	}
}

func TestSourceHandler_PostDuplicada(t *testing.T) {
	repo := newFakeSourceRepo()
	syncer := newSpySyncer()
	r := setupSourceRouter(repo, syncer, t.TempDir())

	body := `{"url":"https://ej.test/dup.m3u"}`
	rec1 := httptest.NewRecorder()
	req1 := httptest.NewRequest(http.MethodPost, "/sources", strings.NewReader(body))
	req1.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusCreated {
		t.Fatalf("1er POST: status = %d, quiero 201", rec1.Code)
	}
	esperarAviso(t, syncer.avisos)

	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPost, "/sources", strings.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusConflict {
		t.Fatalf("2º POST: status = %d, quiero 409", rec2.Code)
	}
}

// construirMultipart arma un cuerpo multipart/form-data con un campo
// "fichero" (y opcionalmente "label"), devolviendo el body y su Content-Type.
func construirMultipart(t *testing.T, contenido []byte, label string) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile("fichero", "mis-canales.m3u")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := fw.Write(contenido); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if label != "" {
		if err := w.WriteField("label", label); err != nil {
			t.Fatalf("WriteField: %v", err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	return &buf, w.FormDataContentType()
}

const m3uEjemplo = "#EXTM3U\n#EXTINF:-1,Canal Uno\nhttp://ej.test/uno.m3u8\n"

func TestSourceHandler_PostMultipartValido(t *testing.T) {
	dir := t.TempDir()
	syncer := newSpySyncer()
	r := setupSourceRouter(newFakeSourceRepo(), syncer, dir)

	body, ct := construirMultipart(t, []byte(m3uEjemplo), "Subida a mano")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/sources", body)
	req.Header.Set("Content-Type", ct)
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, quiero 201 (body=%s)", rec.Code, rec.Body.String())
	}
	var got fuenteJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("respuesta no es JSON: %v", err)
	}
	if got.Kind != "file" {
		t.Errorf("kind = %q, quiero file", got.Kind)
	}
	if got.Label != "Subida a mano" {
		t.Errorf("label = %q, quiero 'Subida a mano'", got.Label)
	}
	if !strings.HasPrefix(got.URL, "file://") {
		t.Fatalf("url = %q, quiero prefijo file://", got.URL)
	}

	path := strings.TrimPrefix(got.URL, "file://")
	if !strings.HasPrefix(path, dir) {
		t.Errorf("el fichero guardado (%q) no está bajo fuentesDir (%q)", path, dir)
	}
	contenido, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("el fichero no se guardó en disco: %v", err)
	}
	if string(contenido) != m3uEjemplo {
		t.Errorf("contenido guardado = %q, quiero %q", contenido, m3uEjemplo)
	}

	esperarAviso(t, syncer.avisos)
}

func TestSourceHandler_PostMultipartSinCampoFichero(t *testing.T) {
	r := setupSourceRouter(newFakeSourceRepo(), newSpySyncer(), t.TempDir())

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("label", "sin fichero")
	_ = w.Close()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/sources", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, quiero 400", rec.Code)
	}
}

func TestSourceHandler_PostMultipartExcedeElTamano(t *testing.T) {
	dir := t.TempDir()
	r := setupSourceRouter(newFakeSourceRepo(), newSpySyncer(), dir)

	grande := bytes.Repeat([]byte("a"), maxTamanoFichero+1)
	body, ct := construirMultipart(t, grande, "")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/sources", body)
	req.Header.Set("Content-Type", ct)
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, quiero 413", rec.Code)
	}

	// Nada debe quedar huérfano en fuentesDir: ni el fichero final ni el
	// temporal de streaming.
	entradas, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entradas) != 0 {
		t.Errorf("fuentesDir no está vacío tras el rechazo: %v", entradas)
	}
}

// TestSourceHandler_PostMultipartContenidoIdenticoEsDuplicado cubre la
// decisión de diseño más novedosa de la tarea: el id de un fichero subido es
// el hash de su CONTENIDO, así que subir el mismo M3U dos veces (aunque el
// segundo envío use otro nombre de fichero) tiene que dar el mismo 409 que
// una URL remota repetida, y fuentesDir no debe acabar con dos copias del
// mismo contenido.
func TestSourceHandler_PostMultipartContenidoIdenticoEsDuplicado(t *testing.T) {
	dir := t.TempDir()
	syncer := newSpySyncer()
	r := setupSourceRouter(newFakeSourceRepo(), syncer, dir)

	body1, ct1 := construirMultipart(t, []byte(m3uEjemplo), "Primera subida")
	rec1 := httptest.NewRecorder()
	req1 := httptest.NewRequest(http.MethodPost, "/sources", body1)
	req1.Header.Set("Content-Type", ct1)
	r.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusCreated {
		t.Fatalf("1ª subida: status = %d, quiero 201 (body=%s)", rec1.Code, rec1.Body.String())
	}
	esperarAviso(t, syncer.avisos)

	// Mismo contenido exacto, segunda subida (nombre de fichero y label
	// distintos: lo que importa es el contenido, no los metadatos).
	body2, ct2 := construirMultipart(t, []byte(m3uEjemplo), "Segunda subida")
	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPost, "/sources", body2)
	req2.Header.Set("Content-Type", ct2)
	r.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusConflict {
		t.Fatalf("2ª subida: status = %d, quiero 409 (body=%s)", rec2.Code, rec2.Body.String())
	}

	entradas, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entradas) != 1 {
		t.Fatalf("fuentesDir tiene %d ficheros, quiero exactamente 1: %v", len(entradas), entradas)
	}
}

// TestSourceHandler_PostMultipartAddFallaLimpiaFicheroHuerfano cubre H1: si
// Add falla por una causa que NO es duplicado, el fichero que guardarFichero
// ya escribió en fuentesDir no debe quedar huérfano.
func TestSourceHandler_PostMultipartAddFallaLimpiaFicheroHuerfano(t *testing.T) {
	dir := t.TempDir()
	repo := newFakeSourceRepo()
	repo.errAdd = errors.New("fake: fallo no relacionado con duplicados")
	r := setupSourceRouter(repo, newSpySyncer(), dir)

	body, ct := construirMultipart(t, []byte(m3uEjemplo), "")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/sources", body)
	req.Header.Set("Content-Type", ct)
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, quiero 500 (body=%s)", rec.Code, rec.Body.String())
	}

	entradas, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entradas) != 0 {
		t.Errorf("fuentesDir tiene ficheros huérfanos tras el fallo de Add: %v", entradas)
	}
}

func TestSourceHandler_DeleteConocida(t *testing.T) {
	repo := newFakeSourceRepo()
	if err := repo.Add(context.Background(), ports.Source{URL: "https://ej.test/del.m3u", Label: "A", Kind: "url", IsActive: true}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	fuentes, _ := repo.List(context.Background())
	id := fuentes[0].ID

	r := setupSourceRouter(repo, newSpySyncer(), t.TempDir())
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/sources/"+id, nil))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, quiero 204", rec.Code)
	}

	restantes, _ := repo.List(context.Background())
	if len(restantes) != 0 {
		t.Errorf("la fuente sigue en el repo tras el DELETE: %v", restantes)
	}
}

func TestSourceHandler_DeleteDesconocida(t *testing.T) {
	r := setupSourceRouter(newFakeSourceRepo(), newSpySyncer(), t.TempDir())
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/sources/no-existe", nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, quiero 404", rec.Code)
	}
}

// TestSourceHandler_DeleteFuenteFicheroBorraElArchivo cubre H2: borrar una
// fuente kind=="file" debe borrar también su M3U de fuentesDir, no solo la
// fila. Sin este comportamiento, altas y bajas repetidas de fuentes por
// fichero hacen crecer el disco sin cota.
func TestSourceHandler_DeleteFuenteFicheroBorraElArchivo(t *testing.T) {
	dir := t.TempDir()
	syncer := newSpySyncer()
	r := setupSourceRouter(newFakeSourceRepo(), syncer, dir)

	body, ct := construirMultipart(t, []byte(m3uEjemplo), "")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/sources", body)
	req.Header.Set("Content-Type", ct)
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST: status = %d, quiero 201 (body=%s)", rec.Code, rec.Body.String())
	}
	esperarAviso(t, syncer.avisos)

	var creada fuenteJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &creada); err != nil {
		t.Fatalf("respuesta no es JSON: %v", err)
	}
	path := strings.TrimPrefix(creada.URL, "file://")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("el fichero no existe tras el alta: %v", err)
	}

	recDel := httptest.NewRecorder()
	r.ServeHTTP(recDel, httptest.NewRequest(http.MethodDelete, "/sources/"+creada.ID, nil))
	if recDel.Code != http.StatusNoContent {
		t.Fatalf("DELETE: status = %d, quiero 204", recDel.Code)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("el fichero sigue en disco tras el DELETE (statErr=%v)", err)
	}
	entradas, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entradas) != 0 {
		t.Errorf("fuentesDir no quedó vacío tras el DELETE: %v", entradas)
	}
}

func TestSourceHandler_SyncConocida(t *testing.T) {
	repo := newFakeSourceRepo()
	if err := repo.Add(context.Background(), ports.Source{URL: "https://ej.test/sync.m3u", Label: "A", Kind: "url", IsActive: true}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	fuentes, _ := repo.List(context.Background())
	id := fuentes[0].ID

	syncer := newSpySyncer()
	r := setupSourceRouter(repo, syncer, t.TempDir())
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/sources/"+id+"/sync", nil))

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, quiero 202", rec.Code)
	}
	got := esperarAviso(t, syncer.avisos)
	if got != id {
		t.Errorf("SyncOne llamado con id=%q, quiero %q", got, id)
	}
}

func TestSourceHandler_SyncDesconocida(t *testing.T) {
	syncer := newSpySyncer()
	r := setupSourceRouter(newFakeSourceRepo(), syncer, t.TempDir())
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/sources/no-existe/sync", nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, quiero 404", rec.Code)
	}
	select {
	case id := <-syncer.avisos:
		t.Errorf("SyncOne no debería haberse llamado (id=%q) para una fuente desconocida", id)
	default:
	}
}

func TestSourceHandler_Sugeridas(t *testing.T) {
	r := setupSourceRouter(newFakeSourceRepo(), newSpySyncer(), t.TempDir())
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/sources/sugeridas", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, quiero 200", rec.Code)
	}
	var got []FuenteSugerida
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("respuesta no es JSON: %v", err)
	}

	quiero := []FuenteSugerida{
		{Label: "IPTV-org (global)", URL: "https://iptv-org.github.io/iptv/index.m3u"},
		{Label: "IPTV-org · México", URL: "https://iptv-org.github.io/iptv/countries/mx.m3u"},
		{Label: "IPTV-org · EE. UU.", URL: "https://iptv-org.github.io/iptv/countries/us.m3u"},
		{Label: "IPTV-org · España", URL: "https://iptv-org.github.io/iptv/countries/es.m3u"},
		{Label: "IPTV-org · Noticias", URL: "https://iptv-org.github.io/iptv/categories/news.m3u"},
		{Label: "IPTV-org · Películas", URL: "https://iptv-org.github.io/iptv/categories/movies.m3u"},
	}
	if len(got) != len(quiero) {
		t.Fatalf("longitud = %d, quiero %d", len(got), len(quiero))
	}
	for i := range quiero {
		if got[i] != quiero[i] {
			t.Errorf("elemento %d = %+v, quiero %+v", i, got[i], quiero[i])
		}
	}
}
