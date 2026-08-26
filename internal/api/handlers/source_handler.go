package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/gdberysan/open-tv/internal/ports"
)

// maxCuerpoFuenteJSON acota el body de POST /sources en su variante JSON: una
// URL y una etiqueta no necesitan más que unos pocos cientos de bytes. Mismo
// patrón que maxCuerpoPlayback en stats_handler.go.
const maxCuerpoFuenteJSON = 4 << 10

// maxTamanoFichero espeja defaultMaxM3UBytes de internal/adapters/providers/opensource:
// el mismo cap de 50MB para un M3U subido a mano que para uno descargado por
// HTTP. No se importa la constante (es privada del paquete opensource) porque
// acoplar los dos paquetes por una sola constante compartida no compensa;
// cualquier cambio en una debe reflejarse aquí a mano.
const maxTamanoFichero = 50 << 20

// sincronizadorPuntual es lo único que SourceHandler necesita del Syncer:
// encender una sincronización bajo demanda para una fuente concreta. Interfaz
// mínima (en vez de acoplarse a *services.Syncer) para que el handler sea
// testeable con un spy que solo registra la llamada.
type sincronizadorPuntual interface {
	SyncOne(ctx context.Context, id string) error
}

// FuenteSugerida es un catálogo curado de fuentes M3U públicas conocidas que
// el cliente puede ofrecer en el onboarding sin que el usuario tenga que salir
// a buscar una URL por su cuenta. Son solo punteros de datos (label + url): no
// llevan canales ni se sincronizan hasta que el usuario decide darlas de alta
// vía POST /sources.
type FuenteSugerida struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}

// fuentesSugeridas es la constante exacta acordada en la spec: 6 fuentes de
// IPTV-org (github.com/iptv-org/iptv), global + 3 países + 2 categorías.
var fuentesSugeridas = []*FuenteSugerida{
	{Label: "IPTV-org (global)", URL: "https://iptv-org.github.io/iptv/index.m3u"},
	{Label: "IPTV-org · México", URL: "https://iptv-org.github.io/iptv/countries/mx.m3u"},
	{Label: "IPTV-org · EE. UU.", URL: "https://iptv-org.github.io/iptv/countries/us.m3u"},
	{Label: "IPTV-org · España", URL: "https://iptv-org.github.io/iptv/countries/es.m3u"},
	{Label: "IPTV-org · Noticias", URL: "https://iptv-org.github.io/iptv/categories/news.m3u"},
	{Label: "IPTV-org · Películas", URL: "https://iptv-org.github.io/iptv/categories/movies.m3u"},
}

// SourceHandler expone /sources*: alta, baja y resync bajo demanda de las
// fuentes IPTV "bring your own" del usuario. repo debe ser el pool de
// ESCRITURA (Add/Remove mutan providers), a diferencia de los handlers de
// /channels que solo leen.
type SourceHandler struct {
	logger     *slog.Logger
	repo       ports.SourceRepository
	syncer     sincronizadorPuntual
	fuentesDir string
}

func NewSourceHandler(logger *slog.Logger, repo ports.SourceRepository, syncer sincronizadorPuntual, fuentesDir string) *SourceHandler {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	return &SourceHandler{logger: logger, repo: repo, syncer: syncer, fuentesDir: fuentesDir}
}

// fuenteJSON es la forma de cable de una fuente, snake_case y exacta a la
// spec: ultimo_sync viaja tal cual (0 = nunca sincronizada), el cliente
// decide cómo mostrarlo.
type fuenteJSON struct {
	ID         string `json:"id"`
	Label      string `json:"label"`
	URL        string `json:"url"`
	Kind       string `json:"kind"`
	UltimoSync int64  `json:"ultimo_sync"`
	Canales    int    `json:"canales"`
}

func aFuenteJSON(s ports.Source) fuenteJSON {
	return fuenteJSON{
		ID:         s.ID,
		Label:      s.Label,
		URL:        s.URL,
		Kind:       s.Kind,
		UltimoSync: s.UltimoSync,
		Canales:    s.Canales,
	}
}

// Get lista todas las fuentes dadas de alta. Ruta: GET /sources
func (h *SourceHandler) Get(w http.ResponseWriter, r *http.Request) {
	fuentes, err := h.repo.List(r.Context())
	if err != nil {
		h.logger.Error("GET /sources: fallo listando fuentes", slog.Any("error", err))
		h.writeError(w, http.StatusInternalServerError, "error obteniendo fuentes")
		return
	}
	salida := make([]fuenteJSON, 0, len(fuentes))
	for _, f := range fuentes {
		salida = append(salida, aFuenteJSON(f))
	}
	h.writeJSON(w, http.StatusOK, salida)
}

// Post da de alta una fuente nueva, en dos variantes según Content-Type:
//   - application/json: {"url","label"?} — una fuente M3U remota.
//   - multipart/form-data con el campo "fichero": un M3U subido a mano, que se
//     guarda en <datadir>/fuentes/<id>.m3u y se registra como "file://<esa ruta>".
//
// Ruta: POST /sources
func (h *SourceHandler) Post(w http.ResponseWriter, r *http.Request) {
	ct := r.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "multipart/form-data") {
		h.postMultipart(w, r)
		return
	}
	h.postJSON(w, r)
}

type crearFuenteBody struct {
	URL   string `json:"url"`
	Label string `json:"label"`
}

func (h *SourceHandler) postJSON(w http.ResponseWriter, r *http.Request) {
	var body crearFuenteBody
	dec := json.NewDecoder(io.LimitReader(r.Body, maxCuerpoFuenteJSON))
	if err := dec.Decode(&body); err != nil {
		h.writeError(w, http.StatusBadRequest, "cuerpo inválido o demasiado grande")
		return
	}

	fuenteURL, err := validarURLFuente(body.URL)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	label := strings.TrimSpace(body.Label)
	if label == "" {
		label = fuenteURL
	}

	h.crearFuente(w, r, fuenteURL, label, "url")
}

// validarURLFuente exige esquema http/https. No es una guarda SSRF sobre la
// descarga en sí (eso lo hace el cliente HTTP del provider, igual que ya
// pasaba con IPTV_ORG_URL): es un gateway loopback de un único usuario que
// descarga un M3U de una URL que ese mismo usuario ha tecleado, mismo modelo
// de confianza que la fuente fija de siempre. Lo que sí se cierra aquí es
// file://: esa vía solo se admite por subida (postMultipart), nunca
// referenciando una ruta arbitraria del disco a través del campo url.
func validarURLFuente(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("url requerida")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "", errors.New("url inválida")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", errors.New("solo se admiten URLs http/https (los ficheros locales se suben, no se referencian por url)")
	}
	return raw, nil
}

// postMultipart procesa la subida de un fichero M3U. Se usa r.MultipartReader
// en vez de ParseMultipartForm: así el cuerpo se procesa en streaming, sin que
// el parseo de multipart llegue a materializar el fichero completo en memoria
// antes de que guardarFichero pueda aplicar el cap de tamaño.
func (h *SourceHandler) postMultipart(w http.ResponseWriter, r *http.Request) {
	mr, err := r.MultipartReader()
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "multipart inválido")
		return
	}

	var (
		fuenteURL string
		label     string
		guardado  bool
	)
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			h.writeError(w, http.StatusBadRequest, "multipart inválido")
			return
		}

		switch part.FormName() {
		case "fichero":
			path, err := h.guardarFichero(part)
			_ = part.Close()
			if err != nil {
				status := http.StatusBadRequest
				if errors.Is(err, errFicheroDemasiadoGrande) {
					status = http.StatusRequestEntityTooLarge
				} else {
					h.logger.Error("POST /sources: fallo guardando el fichero subido", slog.Any("error", err))
				}
				h.writeError(w, status, err.Error())
				return
			}
			fuenteURL = "file://" + path
			guardado = true
		case "label":
			buf, _ := io.ReadAll(io.LimitReader(part, 200))
			_ = part.Close()
			label = strings.TrimSpace(string(buf))
		default:
			_ = part.Close()
		}
	}

	if !guardado {
		h.writeError(w, http.StatusBadRequest, "falta el campo 'fichero' con un M3U")
		return
	}
	if label == "" {
		label = "Fichero subido"
	}

	h.crearFuente(w, r, fuenteURL, label, "file")
}

// errFicheroDemasiadoGrande señala que el cuerpo subido excede maxTamanoFichero.
var errFicheroDemasiadoGrande = errors.New("el fichero supera el tamaño máximo permitido (50MB)")

// guardarFichero copia el contenido de part a un fichero temporal DENTRO de
// fuentesDir, calculando en el mismo paso un hash fnv64a del contenido. El id
// final del fichero (y por tanto su nombre) es ese hash: subir el mismo
// contenido dos veces produce el mismo path, la misma URL "file://" y, por
// tanto, el mismo id de fuente vía sourceID() en el repositorio — lo que
// convierte la subida duplicada en el mismo 409 que una URL remota repetida,
// sin tener que comparar contenidos a mano en este handler.
//
// El cap de tamaño se aplica AL STREAM (io.LimitReader con N+1 para distinguir
// "justo en el límite" de "excedido"), no solo a un parámetro de memoria de
// ParseMultipartForm: un adjunto que iguala el límite no debe rechazarse, uno
// que lo supera en un solo byte sí.
//
// La ruta final siempre queda bajo fuentesDir con un nombre generado aquí
// mismo (el hash): en ningún momento se usa el nombre de fichero que trae el
// cliente para construir un path, así que no hay superficie de path-traversal
// vía ese campo.
func (h *SourceHandler) guardarFichero(part io.Reader) (string, error) {
	tmp, err := os.CreateTemp(h.fuentesDir, "upload-*.tmp")
	if err != nil {
		return "", fmt.Errorf("handlers.guardarFichero (CreateTemp): %w", err)
	}
	tmpPath := tmp.Name()
	renombrado := false
	defer func() {
		if !renombrado {
			_ = os.Remove(tmpPath)
		}
	}()

	hasher := fnv.New64a()
	limited := &io.LimitedReader{R: part, N: maxTamanoFichero + 1}
	if _, err := io.Copy(io.MultiWriter(tmp, hasher), limited); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("handlers.guardarFichero (Copy): %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("handlers.guardarFichero (Close): %w", err)
	}
	if limited.N <= 0 {
		return "", errFicheroDemasiadoGrande
	}

	id := fmt.Sprintf("file-%016x", hasher.Sum64())
	destino := filepath.Join(h.fuentesDir, id+".m3u")
	if err := os.Rename(tmpPath, destino); err != nil {
		return "", fmt.Errorf("handlers.guardarFichero (Rename): %w", err)
	}
	renombrado = true

	abs, err := filepath.Abs(destino)
	if err != nil {
		return "", fmt.Errorf("handlers.guardarFichero (Abs): %w", err)
	}
	return abs, nil
}

// crearFuente es el tramo común a las dos variantes de Post: da de alta la
// fuente, la relocaliza para responder con su forma completa (id, ultimo_sync,
// canales — todos calculados por el repositorio, no por este handler) y
// dispara un sync inicial en segundo plano.
func (h *SourceHandler) crearFuente(w http.ResponseWriter, r *http.Request, fuenteURL, label, kind string) {
	err := h.repo.Add(r.Context(), ports.Source{URL: fuenteURL, Label: label, Kind: kind, IsActive: true})
	if err != nil {
		if errors.Is(err, ports.ErrFuenteDuplicada) {
			// Duplicado NO es huérfano: mismo contenido → mismo path → el
			// fichero que ya está en disco pertenece legítimamente a la
			// fuente ya existente. No se toca.
			h.writeError(w, http.StatusConflict, "ya existe una fuente con esa URL")
			return
		}
		// Cualquier OTRO fallo de Add (DB ocupada, disco lleno...) deja un
		// fichero recién escrito en fuentesDir sin ninguna fila que lo
		// reclame, si esta alta venía de una subida. Se borra aquí, antes de
		// responder, para que un Add fallido no acumule M3Us huérfanos.
		if kind == "file" {
			if path, ok := h.rutaFicheroSubido(fuenteURL); ok {
				h.borrarFicheroSubido(path, "alta fallida")
			}
		}
		h.logger.Error("POST /sources: fallo dando de alta la fuente", slog.Any("error", err))
		h.writeError(w, http.StatusInternalServerError, "no se pudo dar de alta la fuente")
		return
	}

	creada, err := h.buscarPorURL(r.Context(), fuenteURL)
	if err != nil {
		// La fuente SÍ se creó (Add no falló): esto es una inconsistencia rara
		// entre Add y List, no un fallo de validación del cliente.
		h.logger.Error("POST /sources: fuente creada pero no localizada tras el alta",
			slog.String("url", fuenteURL), slog.Any("error", err))
		h.writeError(w, http.StatusInternalServerError, "fuente creada pero no se pudo confirmar")
		return
	}

	// Sync inicial asíncrono: una lista M3U grande puede tardar bastante en
	// descargarse y parsear, y el 201 no debe esperar a eso. El error se
	// registra; el usuario tiene POST /sources/{id}/sync para reintentar a
	// mano. Deliberadamente sin tracking en el WaitGroup de apagado de
	// main.go: es un proceso local de un único usuario, y en el peor caso el
	// proceso termina un poco antes de que este sync puntual acabe (aceptable
	// aquí; no lo sería en un servicio multi-usuario).
	go func(id string) {
		if err := h.syncer.SyncOne(context.Background(), id); err != nil {
			h.logger.Error("Sync inicial de fuente nueva fallido",
				slog.String("fuente", id), slog.Any("error", err))
		}
	}(creada.ID)

	h.writeJSON(w, http.StatusCreated, aFuenteJSON(creada))
}

// rutaFicheroSubido extrae la ruta de disco de una URL "file://<ruta>" y
// valida que quede CONTENIDA en fuentesDir, con la misma disciplina que
// opensource.Provider.resolveFilePath (Abs+Clean en ambos lados y contención
// real por filepath.Rel, nunca un simple strings.HasPrefix sobre las
// cadenas). ok=false si la URL no es file:// o si, por lo que sea, la ruta
// resultante cae fuera de fuentesDir: un borrado nunca debe fiarse a ciegas
// de un campo que, en las fuentes que este propio handler crea, siempre
// debería quedar dentro.
func (h *SourceHandler) rutaFicheroSubido(fuenteURL string) (string, bool) {
	if !strings.HasPrefix(fuenteURL, "file://") || h.fuentesDir == "" {
		return "", false
	}
	raw := strings.TrimPrefix(fuenteURL, "file://")
	if raw == "" {
		return "", false
	}
	allowedAbs, err := filepath.Abs(filepath.Clean(h.fuentesDir))
	if err != nil {
		return "", false
	}
	pathAbs, err := filepath.Abs(filepath.Clean(raw))
	if err != nil {
		return "", false
	}
	rel, err := filepath.Rel(allowedAbs, pathAbs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return pathAbs, true
}

// borrarFicheroSubido borra un fichero ya validado como contenido en
// fuentesDir, tolerando que ya no exista. Un fallo de borrado (permisos,
// I/O) se loguea pero nunca aborta el flujo que lo invoca: en ambos call
// sites (alta fallida, fuente borrada) el peor desenlace de un os.Remove
// fallido es un fichero huérfano benigno, no una inconsistencia del catálogo.
func (h *SourceHandler) borrarFicheroSubido(path, motivo string) {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		h.logger.Warn("No se pudo borrar el fichero subido de una fuente",
			slog.String("motivo", motivo), slog.String("path", path), slog.Any("error", err))
	}
}

// buscarPorURL relee la lista de fuentes para encontrar la que se acaba de dar
// de alta. Evita duplicar en este paquete el cálculo del id (fnv64a de la URL,
// interno a db.SourceRepository) y de paso devuelve valores reales de
// ultimo_sync/canales tal como los calcula el repositorio, no unos supuestos
// a mano en el handler.
func (h *SourceHandler) buscarPorURL(ctx context.Context, fuenteURL string) (ports.Source, error) {
	fuentes, err := h.repo.List(ctx)
	if err != nil {
		return ports.Source{}, fmt.Errorf("handlers.buscarPorURL (List): %w", err)
	}
	for _, f := range fuentes {
		if f.URL == fuenteURL {
			return f, nil
		}
	}
	return ports.Source{}, fmt.Errorf("handlers.buscarPorURL: ninguna fuente con url=%s", fuenteURL)
}

// buscarPorID hace una búsqueda barata a partir de List: no hay un Get-por-id
// en ports.SourceRepository (solo List/Add/Remove/TouchSync) y añadir uno
// solo para esto no compensa frente a reutilizar List, que ya se paga en cada
// GET /sources. Se usa tanto para el 404 de DELETE/Sync como, en DELETE, para
// saber si la fuente es kind=="file" y qué fichero le corresponde borrar.
func (h *SourceHandler) buscarPorID(ctx context.Context, id string) (ports.Source, bool, error) {
	fuentes, err := h.repo.List(ctx)
	if err != nil {
		return ports.Source{}, false, fmt.Errorf("handlers.buscarPorID (List): %w", err)
	}
	for _, f := range fuentes {
		if f.ID == id {
			return f, true, nil
		}
	}
	return ports.Source{}, false, nil
}

// Delete borra una fuente y, en cascada (a cargo del repositorio), sus
// canales y streams. Un id desconocido es un 404: se comprueba por
// adelantado en vez de ejecutar el DELETE a ciegas, porque Remove no informa
// si borró alguna fila.
//
// Para kind=="file", el M3U subido en fuentesDir se borra DESPUÉS de que la
// fila haya desaparecido con éxito, nunca antes: si el orden fuera al revés y
// Remove fallara a mitad, la fuente quedaría con su fila (y su fila en
// providers) viva pero sin el fichero que la respalda — rota de verdad. Con
// fila-primero-fichero-después, el peor caso de un os.Remove fallido es un
// fichero huérfano benigno (se loguea): el catálogo ya quedó consistente,
// porque la fila y sus canales/streams ya se fueron en cascada.
// Ruta: DELETE /sources/{id}
func (h *SourceHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := pathParam(r, "id")
	if id == "" {
		h.writeError(w, http.StatusBadRequest, "id requerido")
		return
	}

	fuente, existe, err := h.buscarPorID(r.Context(), id)
	if err != nil {
		h.logger.Error("DELETE /sources: fallo comprobando existencia", slog.Any("error", err))
		h.writeError(w, http.StatusInternalServerError, "error borrando la fuente")
		return
	}
	if !existe {
		h.writeError(w, http.StatusNotFound, "fuente no encontrada")
		return
	}

	if err := h.repo.Remove(r.Context(), id); err != nil {
		h.logger.Error("DELETE /sources: fallo borrando", slog.String("id", id), slog.Any("error", err))
		h.writeError(w, http.StatusInternalServerError, "error borrando la fuente")
		return
	}

	if fuente.Kind == "file" {
		if path, ok := h.rutaFicheroSubido(fuente.URL); ok {
			h.borrarFicheroSubido(path, "fuente borrada")
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

// Sync dispara un resync bajo demanda de una fuente, en segundo plano (igual
// que el sync inicial de crearFuente: la respuesta no espera a que termine).
// El id desconocido se comprueba por adelantado porque, al ser asíncrono, un
// error de SyncOne ya no tiene forma de llegar como respuesta HTTP.
// Ruta: POST /sources/{id}/sync
func (h *SourceHandler) Sync(w http.ResponseWriter, r *http.Request) {
	id := pathParam(r, "id")
	if id == "" {
		h.writeError(w, http.StatusBadRequest, "id requerido")
		return
	}

	_, existe, err := h.buscarPorID(r.Context(), id)
	if err != nil {
		h.logger.Error("POST /sources/{id}/sync: fallo comprobando existencia", slog.Any("error", err))
		h.writeError(w, http.StatusInternalServerError, "error disparando el sync")
		return
	}
	if !existe {
		h.writeError(w, http.StatusNotFound, "fuente no encontrada")
		return
	}

	// Mismo trade-off que el sync inicial de crearFuente: goroutine sin
	// tracking en el WaitGroup de apagado de main.go, aceptable en un proceso
	// local de un único usuario.
	go func(id string) {
		if err := h.syncer.SyncOne(context.Background(), id); err != nil {
			h.logger.Error("Sync manual fallido", slog.String("fuente", id), slog.Any("error", err))
		}
	}(id)
	w.WriteHeader(http.StatusAccepted)
}

// Sugeridas devuelve el catálogo curado de fuentes públicas conocidas.
// Ruta: GET /sources/sugeridas
func (h *SourceHandler) Sugeridas(w http.ResponseWriter, r *http.Request) {
	h.writeJSON(w, http.StatusOK, fuentesSugeridas)
}

func (h *SourceHandler) writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (h *SourceHandler) writeError(w http.ResponseWriter, status int, msg string) {
	h.writeJSON(w, status, map[string]string{"error": msg})
}
