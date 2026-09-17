# Modo red con clave de acceso + imagen de Docker — plan de implementación

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Que Open TV pueda correr en Docker y verse desde cualquier dispositivo de la red, exigiendo una clave de acceso fuera de loopback, con un proxy HLS que deja de ser un relé abierto, y publicado como imagen multi-arquitectura en `ghcr.io` para v1.1.0.

**Architecture:** Un paquete nuevo `internal/acceso` (clave, sesiones HMAC sin estado, limitador de intentos, middleware y página de acceso renderizada en servidor, sin JS). El proxy (`internal/proxy`) solo relaya URLs que están en el catálogo o que él mismo firmó con HMAC al reescribir un manifiesto; esto aplica en los dos modos. `cmd/open-tv` decide el modo por la IP real del listener: loopback = como hoy; cualquier otra = modo red con clave obligatoria. La imagen se construye con goreleaser (`dockers_v2`) a partir de los binarios del release.

**Tech Stack:** Go 1.x stdlib (`crypto/hmac`, `crypto/sha256`, `crypto/rand`, `crypto/subtle`), chi v5, SQLite (modernc), Svelte 5 + Vitest, Playwright, goreleaser v2 `dockers_v2`, distroless.

**Spec:** `docs/superpowers/specs/2026-09-17-imagen-docker-design.md` (aprobada por el dueño el 2026-09-17).

## Global Constraints

- **Ninguna dependencia Go/JS nueva.** Todo con stdlib y lo que ya hay.
- **`mobile/` CERO diffs.** No alterar `/channels` ni `/sources` (contrato de 15 claves).
- **En loopback nada cambia para el usuario:** sin clave, sin pantalla de acceso, mismo comportamiento de reproducción.
- **Sin autenticación solo en loopback;** cualquier listener no-loopback exige sesión en todo salvo `GET /health` y `GET|POST /acceso`.
- **El proxy solo relaya** URLs del catálogo (`streams.url` exacta) o URLs con firma HMAC válida emitida por el propio proceso.
- **No añadir regiones `aria-live` al cliente Svelte.** La página de acceso es HTML de servidor, fuera de App.
- **CSP estricta:** la página de acceso no lleva scripts; su CSP es `default-src 'none'; style-src 'unsafe-inline'; form-action 'self'; frame-ancestors 'none'; base-uri 'none'`.
- **Artefactos en español** (código, comentarios, commits); la página de acceso en español e inglés según `Accept-Language`.
- **Commits:** autor `Gerard <gdberysan@gmail.com>`, trailer `Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>`. **NUNCA `git add -A`**, rutas explícitas.
- **Gates al final de CADA tarea:** `gofmt -l .` (vacío) · `go vet ./...` · `go build ./...` · `go test -race -count=1 ./...` · `golangci-lint run ./...` · `go run ./tools/scrubcheck`. Si la tarea toca `web/`: `cd web && npm run check && npm test && npm run build`, y después restaurar `internal/ui/dist/.gitkeep` (`git checkout -- internal/ui/dist/.gitkeep`).
- **Nombres de variables de entorno nuevas:** `OPEN_TV_ACCESS_KEY`. Nombre de cookie: `open_tv_sesion`. Fichero de clave: `<directorio de la DB>/access-key`, permisos `0600`.
- **Rama:** `feat/modo-red-docker`. Nada a `main` sin autorización del dueño.

---

## Estructura de ficheros

| Fichero | Responsabilidad |
|---|---|
| `internal/proxy/firma.go` (nuevo) | `Firmador`: HMAC-SHA256 de URLs del proxy |
| `internal/proxy/manifiesto.go` | Reescritura que añade `&f=<firma>` a cada URL hija |
| `internal/proxy/handler.go` | Autoriza cada petición: firma válida o URL del catálogo |
| `internal/ports/stream_repository.go` | Puerto `VerificadorURLs` |
| `internal/adapters/db/stream_repository.go` | `ExisteURL` |
| `internal/acceso/clave.go` (nuevo) | Cargar o generar la clave |
| `internal/acceso/sesion.go` (nuevo) | Tokens de sesión HMAC sin estado, comparación de clave |
| `internal/acceso/limite.go` (nuevo) | Limitador de intentos por IP |
| `internal/acceso/http.go` (nuevo) | Middleware `Exigir` y handler de `/acceso` |
| `internal/acceso/pagina.go` (nuevo) | HTML de la página de acceso (es/en) |
| `internal/api/middleware/mismo_origen.go` | Comprobación de `Origin` en métodos mutantes |
| `internal/api/router.go` | Opciones `Firmador`, `Catalogo`, `Sesiones`, `Limitador` |
| `cmd/open-tv/main.go` | Modo red, clave, proxy siempre montado |
| `cmd/open-tv/subcomandos.go` (nuevo) | `open-tv access-key` y `open-tv healthcheck` |
| `web/src/datos/http.ts` | 401 → ir a `/acceso` |
| `web/tests/e2e/global-setup.ts`, `web/tests/e2e/modo-red.spec.ts` (nuevo) | e2e del modo red |
| `Dockerfile`, `.dockerignore`, `compose.yaml` (nuevos) | Imagen desde el código y ejemplo de despliegue |
| `.goreleaser.yml`, `.github/workflows/release.yml`, `.github/workflows/ci.yml` | Publicación en ghcr.io y humo de Docker en CI |
| `README.md`, `README.es.md`, `SECURITY.md`, `CONTRIBUTING.md`, `CLAUDE.md`, `CHANGELOG.md`, spec | Documentación |

---

### Task 1: Proxy con URLs firmadas y autorización por catálogo

**Files:**
- Create: `internal/proxy/firma.go`
- Create: `internal/proxy/firma_test.go`
- Modify: `internal/proxy/manifiesto.go`
- Modify: `internal/proxy/handler.go`
- Modify: `internal/proxy/manifiesto_test.go` (llamadas existentes pasan `nil` como firmador)
- Modify: `internal/proxy/handler_test.go`, `internal/proxy/handler_internal_test.go` (llamadas existentes a `NewHandler`)
- Modify: `internal/api/router.go` (solo la llamada a `proxy.NewHandler`, para que compile)

**Interfaces:**
- Produces:
  - `type Firmador struct{...}`; `func NuevoFirmador(clave []byte) (*Firmador, error)` (error si `len(clave) < 32`); `func NuevoFirmadorAleatorio() (*Firmador, error)`; `func (f *Firmador) Firmar(u string) string`; `func (f *Firmador) Valida(u, firma string) bool` (nil-safe: nil → false).
  - `func ReescribirManifiesto(base *url.URL, cuerpo, prefijo string, firmar func(string) string) string` — si `firmar != nil`, cada URL envuelta termina en `&f=<firmar(urlAbsoluta)>`.
  - `func NewHandler(prefijo string, firmador *Firmador, permitirDestinosPrivados bool, opts ...Option) *Handler`
  - `func ConCatalogo(enCatalogo func(ctx context.Context, url string) bool) Option`
  - Respuesta `403 "destino no autorizado"` cuando ni la firma ni el catálogo autorizan.

- [ ] **Step 1: Test del firmador**

`internal/proxy/firma_test.go`:

```go
package proxy_test

import (
	"bytes"
	"testing"

	"github.com/gdberysan/open-tv/internal/proxy"
)

func firmadorDePrueba(t *testing.T) *proxy.Firmador {
	t.Helper()
	f, err := proxy.NuevoFirmador(bytes.Repeat([]byte{7}, 32))
	if err != nil {
		t.Fatalf("NuevoFirmador: %v", err)
	}
	return f
}

func TestFirmadorValidaSoloSuPropiaFirma(t *testing.T) {
	f := firmadorDePrueba(t)
	u := "https://origen.example/live/seg1.ts"
	firma := f.Firmar(u)

	if !f.Valida(u, firma) {
		t.Fatal("la firma recién emitida no valida")
	}
	if f.Valida("https://origen.example/live/seg2.ts", firma) {
		t.Error("una firma vale para otra URL")
	}
	if f.Valida(u, firma[:len(firma)-1]+"A") && firma[len(firma)-1] != 'A' {
		t.Error("una firma manipulada valida")
	}
	if f.Valida(u, "") {
		t.Error("firma vacía valida")
	}

	otro, err := proxy.NuevoFirmador(bytes.Repeat([]byte{8}, 32))
	if err != nil {
		t.Fatalf("NuevoFirmador: %v", err)
	}
	if otro.Valida(u, firma) {
		t.Error("la firma de un proceso vale en otro con otra clave")
	}
}

func TestFirmadorNilNoValidaNada(t *testing.T) {
	var f *proxy.Firmador
	if f.Valida("https://x/y", "loquesea") {
		t.Error("un firmador nil valida")
	}
}

func TestNuevoFirmadorExigeClaveLarga(t *testing.T) {
	if _, err := proxy.NuevoFirmador(make([]byte, 31)); err == nil {
		t.Error("una clave de 31 bytes se aceptó")
	}
}

func TestFirmadorAleatorioEsDistintoCadaVez(t *testing.T) {
	a, err := proxy.NuevoFirmadorAleatorio()
	if err != nil {
		t.Fatalf("NuevoFirmadorAleatorio: %v", err)
	}
	b, err := proxy.NuevoFirmadorAleatorio()
	if err != nil {
		t.Fatalf("NuevoFirmadorAleatorio: %v", err)
	}
	if a.Firmar("https://x/y") == b.Firmar("https://x/y") {
		t.Error("dos firmadores aleatorios firman igual")
	}
}
```

- [ ] **Step 2: Verificar que falla**

Run: `go test ./internal/proxy/ -run 'Firmador' -count=1`
Expected: FAIL de compilación (`undefined: proxy.NuevoFirmador`).

- [ ] **Step 3: Implementar `firma.go`**

```go
package proxy

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

// Firmador firma las URLs que el propio proxy emite al reescribir un
// manifiesto. Sin firma, /proxy/hls?u= relayaría cualquier URL pública que se
// le pidiera: un relé abierto con la IP de quien lo levanta. Con ella, solo
// pasan las URLs del catálogo (comprobadas aparte) y las que este mismo
// proceso sacó de un manifiesto que ya había autorizado.
type Firmador struct {
	clave []byte
}

// NuevoFirmador copia la clave: quien la pasa no puede cambiarla por debajo.
func NuevoFirmador(clave []byte) (*Firmador, error) {
	if len(clave) < 32 {
		return nil, fmt.Errorf("proxy.NuevoFirmador: la clave necesita al menos 32 bytes, tiene %d", len(clave))
	}
	c := make([]byte, len(clave))
	copy(c, clave)
	return &Firmador{clave: c}, nil
}

// NuevoFirmadorAleatorio es el de producción: una clave nueva por proceso. Al
// reiniciar, las URLs hijas ya emitidas dejan de valer, y el reproductor las
// recupera volviendo a pedir el manifiesto, que es del catálogo.
func NuevoFirmadorAleatorio() (*Firmador, error) {
	clave := make([]byte, 32)
	if _, err := rand.Read(clave); err != nil {
		return nil, fmt.Errorf("proxy.NuevoFirmadorAleatorio: %w", err)
	}
	return &Firmador{clave: clave}, nil
}

// Firmar devuelve el HMAC-SHA256 de u en base64url sin relleno, que viaja tal
// cual en una query sin escapar.
func (f *Firmador) Firmar(u string) string {
	mac := hmac.New(sha256.New, f.clave)
	_, _ = mac.Write([]byte(u))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// Valida compara en tiempo constante. Un firmador nil no valida nada: el
// proxy sin firmador solo acepta lo que esté en el catálogo.
func (f *Firmador) Valida(u, firma string) bool {
	if f == nil || firma == "" {
		return false
	}
	return hmac.Equal([]byte(f.Firmar(u)), []byte(firma))
}
```

- [ ] **Step 4: Verificar que pasa**

Run: `go test ./internal/proxy/ -run 'Firmador' -count=1`
Expected: PASS.

- [ ] **Step 5: Test de reescritura con firma**

Añadir a `internal/proxy/manifiesto_test.go` (y cambiar las cinco llamadas existentes `proxy.ReescribirManifiesto(base(t), entrada, prefijo)` por `proxy.ReescribirManifiesto(base(t), entrada, prefijo, nil)`):

```go
func TestReescribirManifiestoFirmaCadaURL(t *testing.T) {
	f := firmadorDePrueba(t)
	entrada := "#EXTM3U\n#EXT-X-KEY:METHOD=AES-128,URI=\"clave.bin\"\n#EXTINF:6.0,\nseg1.ts\n"

	got := proxy.ReescribirManifiesto(base(t), entrada, prefijo, f.Firmar)

	for _, rel := range []string{"clave.bin", "seg1.ts"} {
		abs, err := base(t).Parse(rel)
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		quiero := prefijo + url.QueryEscape(abs.String()) + "&f=" + f.Firmar(abs.String())
		if !strings.Contains(got, quiero) {
			t.Errorf("falta %q en:\n%s", quiero, got)
		}
	}
}
```

Si `manifiesto_test.go` no importa `net/url` o `strings`, añadirlos.

- [ ] **Step 6: Verificar que falla**

Run: `go test ./internal/proxy/ -run 'ReescribirManifiesto' -count=1`
Expected: FAIL de compilación (demasiados argumentos).

- [ ] **Step 7: Implementar la firma en `manifiesto.go`**

Cambiar las tres funciones para que propaguen `firmar`:

```go
func ReescribirManifiesto(base *url.URL, cuerpo, prefijo string, firmar func(string) string) string {
	lineas := strings.Split(cuerpo, "\n")
	for i, linea := range lineas {
		recortada := strings.TrimSpace(linea)
		switch {
		case recortada == "":
			// Se deja como está, con su \r si lo tenía.
		case strings.HasPrefix(recortada, "#"):
			lineas[i] = reescribirAtributoURI(base, linea, prefijo, firmar)
		default:
			lineas[i] = envolver(base, recortada, prefijo, firmar)
		}
	}
	return strings.Join(lineas, "\n")
}

func reescribirAtributoURI(base *url.URL, linea, prefijo string, firmar func(string) string) string {
	const marca = `URI="`
	i := strings.Index(linea, marca)
	if i < 0 {
		return linea
	}
	inicio := i + len(marca)
	fin := strings.Index(linea[inicio:], `"`)
	if fin < 0 {
		return linea
	}
	valor := linea[inicio : inicio+fin]
	return linea[:inicio] + envolver(base, valor, prefijo, firmar) + linea[inicio+fin:]
}

// envolver resuelve ref contra base, la mete en el prefijo del proxy y, si hay
// firmador, le pega la firma de la URL absoluta: es la que el handler vuelve
// a calcular sobre el parámetro u al recibirla.
func envolver(base *url.URL, ref, prefijo string, firmar func(string) string) string {
	abs, err := base.Parse(ref)
	if err != nil {
		return ref
	}
	destino := abs.String()
	salida := prefijo + url.QueryEscape(destino)
	if firmar != nil {
		salida += "&f=" + firmar(destino)
	}
	return salida
}
```

Actualizar también el comentario de paquete de `manifiesto.go`: sustituir «Solo se monta si la dirección de escucha es loopback, y nunca forma parte del sitio hospedado.» por «Solo relaya URLs del catálogo o firmadas por el propio proceso (ver firma.go), y en modo red además exige sesión.»

- [ ] **Step 8: Tests de autorización del handler**

Añadir a `internal/proxy/handler_test.go`:

```go
func origenContado(t *testing.T) (*httptest.Server, *int) {
	t.Helper()
	pedidas := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pedidas++
		if strings.HasSuffix(r.URL.Path, ".m3u8") {
			w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
			_, _ = w.Write([]byte("#EXTM3U\n#EXTINF:6.0,\nseg1.ts\n"))
			return
		}
		_, _ = w.Write([]byte("segmento"))
	}))
	t.Cleanup(srv.Close)
	return srv, &pedidas
}

func pedirProxy(h http.Handler, destino, firma string) *httptest.ResponseRecorder {
	ruta := "/proxy/hls?u=" + url.QueryEscape(destino)
	if firma != "" {
		ruta += "&f=" + firma
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, ruta, nil))
	return rec
}

func nadaEnCatalogo(context.Context, string) bool { return false }

func TestProxyRechazaURLSinFirmaFueraDelCatalogo(t *testing.T) {
	origen, pedidas := origenContado(t)
	h := proxy.NewHandler("/proxy/hls?u=", firmadorDePrueba(t), true, proxy.ConCatalogo(nadaEnCatalogo))

	rec := pedirProxy(h, origen.URL+"/cualquier.ts", "")

	if rec.Code != http.StatusForbidden {
		t.Fatalf("código %d, quiero 403", rec.Code)
	}
	if *pedidas != 0 {
		t.Errorf("el origen recibió %d peticiones: el proxy relayó antes de autorizar", *pedidas)
	}
}

func TestProxyAceptaURLDelCatalogo(t *testing.T) {
	origen, _ := origenContado(t)
	manifiesto := origen.URL + "/canal.m3u8"
	h := proxy.NewHandler("/proxy/hls?u=", firmadorDePrueba(t), true,
		proxy.ConCatalogo(func(_ context.Context, u string) bool { return u == manifiesto }))

	if rec := pedirProxy(h, manifiesto, ""); rec.Code != http.StatusOK {
		t.Fatalf("código %d, quiero 200: %s", rec.Code, rec.Body.String())
	}
}

func TestProxyAceptaURLFirmadaYRechazaFirmaAjena(t *testing.T) {
	origen, _ := origenContado(t)
	f := firmadorDePrueba(t)
	h := proxy.NewHandler("/proxy/hls?u=", f, true, proxy.ConCatalogo(nadaEnCatalogo))
	seg := origen.URL + "/seg1.ts"

	if rec := pedirProxy(h, seg, f.Firmar(seg)); rec.Code != http.StatusOK {
		t.Errorf("firmada: código %d, quiero 200", rec.Code)
	}
	if rec := pedirProxy(h, seg, f.Firmar(origen.URL+"/otro.ts")); rec.Code != http.StatusForbidden {
		t.Errorf("firma de otra URL: código %d, quiero 403", rec.Code)
	}
}

// El camino completo: un manifiesto del catálogo sale reescrito con URLs
// firmadas, y esas URLs, pedidas tal cual, pasan.
func TestProxyLasURLsDelManifiestoReescritoSonAceptadas(t *testing.T) {
	origen, _ := origenContado(t)
	manifiesto := origen.URL + "/canal.m3u8"
	h := proxy.NewHandler("/proxy/hls?u=", firmadorDePrueba(t), true,
		proxy.ConCatalogo(func(_ context.Context, u string) bool { return u == manifiesto }))

	rec := pedirProxy(h, manifiesto, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("manifiesto: código %d", rec.Code)
	}
	var hija string
	for _, linea := range strings.Split(rec.Body.String(), "\n") {
		if strings.HasPrefix(linea, "/proxy/hls?u=") {
			hija = linea
		}
	}
	if !strings.Contains(hija, "&f=") {
		t.Fatalf("la URL hija no lleva firma: %q", hija)
	}
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, hija, nil))
	if rec2.Code != http.StatusOK {
		t.Errorf("URL hija: código %d, quiero 200", rec2.Code)
	}
}
```

Añadir `"context"` a los imports de `handler_test.go`.

Cambiar TODAS las llamadas existentes a `NewHandler` en `handler_test.go` de `proxy.NewHandler("/proxy/hls?u=", X)` a `proxy.NewHandler("/proxy/hls?u=", firmadorDePrueba(t), X, proxy.ConCatalogo(todoEnCatalogo))`, con este helper en el mismo fichero:

```go
// todoEnCatalogo mantiene el significado de los tests anteriores a la firma:
// prueban el relay, no la autorización.
func todoEnCatalogo(context.Context, string) bool { return true }
```

En `handler_internal_test.go` (paquete `proxy`, no `proxy_test`), cambiar `NewHandler("/proxy/hls?u=", true, …)` por `NewHandler("/proxy/hls?u=", firmadorInterno(t), true, ConCatalogo(func(context.Context, string) bool { return true }), …)` con:

```go
func firmadorInterno(t *testing.T) *Firmador {
	t.Helper()
	f, err := NuevoFirmador(bytes.Repeat([]byte{7}, 32))
	if err != nil {
		t.Fatalf("NuevoFirmador: %v", err)
	}
	return f
}
```

(importar `bytes` y `context` si faltan).

- [ ] **Step 9: Verificar que falla**

Run: `go test ./internal/proxy/ -count=1`
Expected: FAIL de compilación (`NewHandler` con demasiados argumentos, `undefined: proxy.ConCatalogo`).

- [ ] **Step 10: Implementar la autorización en `handler.go`**

Añadir campos y opción:

```go
// ConCatalogo conecta la consulta "¿esta URL es un stream del catálogo?". Sin
// ella, solo pasan las URLs firmadas: el zero value es el seguro.
func ConCatalogo(enCatalogo func(ctx context.Context, url string) bool) Option {
	return func(h *Handler) { h.enCatalogo = enCatalogo }
}
```

En `type Handler struct`, añadir:

```go
	// firmador firma las URLs hijas al reescribir y valida las que vuelven.
	firmador *Firmador
	// enCatalogo autoriza las URLs de nivel superior, que el cliente construye
	// con la URL cruda del mirror y por eso nunca llevan firma.
	enCatalogo func(ctx context.Context, url string) bool
```

Nueva firma del constructor y comentario:

```go
// NewHandler construye el proxy. prefijo es la ruta con la que se reescriben
// las URIs del manifiesto, normalmente "/proxy/hls?u=". firmador firma las
// URLs hijas; con él nil, las hijas no llevarían firma y solo pasaría lo que
// esté en el catálogo.
//
// permitirDestinosPrivados solo es true en tests: los servidores de httptest
// viven en 127.0.0.1, que en producción es exactamente lo que hay que
// bloquear. En el router se monta siempre con false.
func NewHandler(prefijo string, firmador *Firmador, permitirDestinosPrivados bool, opts ...Option) *Handler {
	h := &Handler{
		prefijo:    prefijo,
		privadasOK: permitirDestinosPrivados,
		firmador:   firmador,
	}
```

En `ServeHTTP`, justo DESPUÉS del bloque de `Sec-Fetch-Site` y ANTES de `ctx, cancel := context.WithTimeout(...)`:

```go
	// Autorización antes de tocar la red: ni una resolución DNS para una URL
	// que el proxy no tiene por qué relayar.
	if !h.autorizado(r.Context(), crudo, r.URL.Query().Get("f")) {
		http.Error(w, "destino no autorizado", http.StatusForbidden)
		return
	}
```

Y el método:

```go
// autorizado: la firma se comprueba primero porque es gratis; el catálogo
// cuesta una consulta a SQLite y solo lo necesitan las URLs de nivel superior.
func (h *Handler) autorizado(ctx context.Context, crudo, firma string) bool {
	if h.firmador.Valida(crudo, firma) {
		return true
	}
	return h.enCatalogo != nil && h.enCatalogo(ctx, crudo)
}
```

En `relayarManifiesto`, sustituir la llamada:

```go
	var firmar func(string) string
	if h.firmador != nil {
		firmar = h.firmador.Firmar
	}
	salida := ReescribirManifiesto(base, string(cuerpo), h.prefijo, firmar)
```

- [ ] **Step 11: Que el router compile**

En `internal/api/router.go`, la llamada existente pasa a (el cableado real del catálogo llega en la Tarea 2):

```go
		firmador, err := proxy.NuevoFirmadorAleatorio()
		if err != nil {
			// crypto/rand no falla en ningún sistema soportado; si lo hiciera,
			// mejor sin proxy que con uno sin firma.
			logger.Error("proxy desactivado: sin fuente de aleatoriedad", slog.Any("error", err))
			r.Get("/proxy/hls", func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) })
		} else {
			ph := proxy.NewHandler(RutaProxy, firmador, opts.PermitirDestinosPrivados,
				proxy.ConBuscadorCabeceras(func(ctx context.Context, u string) (string, string) {
					ref, ua, err := streams.CabecerasPorURL(ctx, u)
					if err != nil {
						// Un fallo de lectura no puede tumbar la reproducción:
						// se cae a las cabeceras de siempre.
						return "", ""
					}
					return ref, ua
				}))
			r.Get("/proxy/hls", ph.ServeHTTP)
		}
```

- [ ] **Step 12: Verificar que pasa**

Run: `go test -race -count=1 ./internal/proxy/ ./internal/api/`
Expected: PASS.

- [ ] **Step 13: Gates Go y commit**

Run los gates Go de Global Constraints. Luego:

```bash
git add internal/proxy/firma.go internal/proxy/firma_test.go internal/proxy/manifiesto.go internal/proxy/manifiesto_test.go internal/proxy/handler.go internal/proxy/handler_test.go internal/proxy/handler_internal_test.go internal/api/router.go
git commit -m "feat(proxy): solo relaya URLs firmadas o del catálogo

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 2: El catálogo autoriza las URLs de nivel superior

**Files:**
- Modify: `internal/ports/stream_repository.go`
- Modify: `internal/adapters/db/stream_repository.go`
- Test: `internal/adapters/db/stream_repository_test.go`
- Modify: `internal/api/router.go`
- Test: `internal/api/router_test.go`
- Modify: `cmd/open-tv/main.go`

**Interfaces:**
- Consumes: `proxy.NewHandler`, `proxy.ConCatalogo`, `proxy.NuevoFirmadorAleatorio` (Tarea 1).
- Produces:
  - `type VerificadorURLs interface { ExisteURL(ctx context.Context, url string) (bool, error) }` en `ports`.
  - `func (r *SQLiteStreamRepository) ExisteURL(ctx context.Context, urlCruda string) (bool, error)`.
  - `api.Options.Catalogo ports.VerificadorURLs` y `api.Options.Firmador *proxy.Firmador` (nil → el router crea uno aleatorio).

- [ ] **Step 1: Test del repositorio**

Añadir a `internal/adapters/db/stream_repository_test.go`:

```go
func TestStreamExisteURL(t *testing.T) {
	ctx := context.Background()
	base := nuevaDBDePrueba(t)
	canales := db.NewChannelRepository(base)
	streams := db.NewStreamRepository(base)

	if err := canales.SaveBatch(ctx, []domain.Channel{
		{ID: "p1-c1", Name: "C1", ProviderID: "p1", ProviderType: domain.ProviderOpenSource},
	}); err != nil {
		t.Fatalf("SaveBatch canales: %v", err)
	}
	if err := streams.SaveBatch(ctx, []domain.Stream{
		{ID: "st-1", ChannelID: "p1-c1", URL: "https://origen/canal.m3u8", Protocol: domain.ProtocolHLS},
	}); err != nil {
		t.Fatalf("SaveBatch: %v", err)
	}

	for _, c := range []struct {
		url   string
		quiero bool
	}{
		{"https://origen/canal.m3u8", true},
		{"https://origen/canal.m3u8?x=1", false},
		{"https://origen/seg1.ts", false},
		{"", false},
	} {
		got, err := streams.ExisteURL(ctx, c.url)
		if err != nil {
			t.Fatalf("ExisteURL(%q): %v", c.url, err)
		}
		if got != c.quiero {
			t.Errorf("ExisteURL(%q) = %v, quiero %v", c.url, got, c.quiero)
		}
	}
}
```

- [ ] **Step 2: Verificar que falla**

Run: `go test ./internal/adapters/db/ -run TestStreamExisteURL -count=1`
Expected: FAIL (`streams.ExisteURL undefined`).

- [ ] **Step 3: Puerto e implementación**

En `internal/ports/stream_repository.go`, al final:

```go
// VerificadorURLs responde si una URL es, exacta, la de un stream del
// catálogo. Lo usa el proxy HLS para autorizar las URLs de nivel superior,
// que el cliente construye con la URL cruda del mirror y no llevan firma. Va
// aparte de StreamRepository para no obligar a todos sus dobles de test a
// implementarlo.
type VerificadorURLs interface {
	ExisteURL(ctx context.Context, url string) (bool, error)
}
```

En `internal/adapters/db/stream_repository.go`, junto a `CabecerasPorURL`:

```go
// ExisteURL usa el mismo índice idx_streams_url que CabecerasPorURL. Solo la
// URL exacta cuenta: los segmentos y claves de un manifiesto nunca son filas
// propias, y esos los autoriza la firma del proxy, no el catálogo.
func (r *SQLiteStreamRepository) ExisteURL(ctx context.Context, urlCruda string) (bool, error) {
	if urlCruda == "" {
		return false, nil
	}
	var uno int
	err := r.db.QueryRowContext(ctx, "SELECT 1 FROM streams WHERE url = ? LIMIT 1", urlCruda).Scan(&uno)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("db.Stream.ExisteURL: %w", err)
	}
	return true, nil
}
```

- [ ] **Step 4: Verificar que pasa**

Run: `go test ./internal/adapters/db/ -run TestStreamExisteURL -count=1`
Expected: PASS.

- [ ] **Step 5: Test del router**

Añadir a `internal/api/router_test.go`:

```go
type catalogoFijo map[string]bool

func (c catalogoFijo) ExisteURL(_ context.Context, u string) (bool, error) { return c[u], nil }

// Una URL que no es del catálogo ni lleva firma no se relaya: el proxy ya no
// es un relé abierto. Se comprueba por el código, sin red: el 403 llega antes
// de resolver nada.
func TestProxyNoRelayaURLsAjenasAlCatalogo(t *testing.T) {
	r := api.NewRouter(slog.New(slog.DiscardHandler), repoVacio{}, provVacio{}, streamsVacio{}, nil, syncVacio{}, sourcesVacio{}, t.TempDir(), nil,
		api.Options{ProxyActivo: true, Catalogo: catalogoFijo{"https://origen.example/canal.m3u8": true}})

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/proxy/hls?u="+url.QueryEscape("https://atacante.example/x"), nil))
	if rec.Code != http.StatusForbidden {
		t.Errorf("URL ajena = %d, quiero 403", rec.Code)
	}
}
```

Añadir `"net/url"` a los imports de `router_test.go` si falta.

- [ ] **Step 6: Verificar que falla**

Run: `go test ./internal/api/ -run TestProxyNoRelayaURLsAjenasAlCatalogo -count=1`
Expected: FAIL de compilación (`unknown field Catalogo`).

- [ ] **Step 7: Cablear en el router**

En `api.Options`, añadir:

```go
	// Catalogo autoriza en el proxy las URLs de nivel superior. nil (tests que
	// no lo necesitan) deja pasar solo URLs firmadas, que es el caso seguro.
	Catalogo ports.VerificadorURLs
	// Firmador firma las URLs hijas del proxy. nil: el router crea uno
	// aleatorio. cmd/open-tv lo crea él para poder fallar al arrancar si no hay
	// aleatoriedad, en vez de degradar en silencio.
	Firmador *proxy.Firmador
```

Sustituir el bloque de la Tarea 1 dentro de `if opts.ProxyActivo {` por:

```go
		firmador := opts.Firmador
		if firmador == nil {
			var err error
			firmador, err = proxy.NuevoFirmadorAleatorio()
			if err != nil {
				// crypto/rand no falla en ningún sistema soportado; si lo
				// hiciera, mejor sin proxy que con uno sin firma.
				logger.Error("proxy desactivado: sin fuente de aleatoriedad", slog.Any("error", err))
			}
		}
		if firmador == nil {
			r.Get("/proxy/hls", func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) })
		} else {
			opciones := []proxy.Option{
				proxy.ConBuscadorCabeceras(func(ctx context.Context, u string) (string, string) {
					ref, ua, err := streams.CabecerasPorURL(ctx, u)
					if err != nil {
						// Un fallo de lectura no puede tumbar la reproducción:
						// se cae a las cabeceras de siempre.
						return "", ""
					}
					return ref, ua
				}),
			}
			if opts.Catalogo != nil {
				catalogo := opts.Catalogo
				opciones = append(opciones, proxy.ConCatalogo(func(ctx context.Context, u string) bool {
					ok, err := catalogo.ExisteURL(ctx, u)
					if err != nil {
						logger.Warn("proxy: no se pudo consultar el catálogo", slog.Any("error", err))
						return false
					}
					return ok
				}))
			}
			ph := proxy.NewHandler(RutaProxy, firmador, opts.PermitirDestinosPrivados, opciones...)
			r.Get("/proxy/hls", ph.ServeHTTP)
		}
```

Actualizar el comentario encima de `if opts.ProxyActivo {`:

```go
	// El proxy HLS solo relaya URLs del catálogo o firmadas por este proceso
	// (internal/proxy/firma.go), así que no es un relé abierto. En modo red,
	// además, todo lo que hay detrás del middleware de acceso exige sesión.
	// ProxyActivo sigue existiendo para los tests que prueban el router sin él.
```

En `cmd/open-tv/main.go`, dentro de `api.Options{...}`, añadir `Catalogo: streamRepoRO,`.

- [ ] **Step 8: Verificar que pasa**

Run: `go test -race -count=1 ./internal/api/ ./internal/adapters/db/ ./cmd/open-tv/`
Expected: PASS.

- [ ] **Step 9: e2e del proxy sigue verde**

El e2e existente reproduce «Canal Con CORS» por el proxy: su URL está en el catálogo y los segmentos salen firmados.

Run: `cd web && npx playwright test tests/e2e/reproduccion.spec.ts --project=chromium; cd .. && git checkout -- internal/ui/dist/.gitkeep`
Expected: PASS.

- [ ] **Step 10: Gates y commit**

Gates Go. Luego:

```bash
git add internal/ports/stream_repository.go internal/adapters/db/stream_repository.go internal/adapters/db/stream_repository_test.go internal/api/router.go internal/api/router_test.go cmd/open-tv/main.go
git commit -m "feat(proxy): el catálogo autoriza las URLs de nivel superior

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 3: Clave de acceso, sesiones y limitador

**Files:**
- Create: `internal/acceso/clave.go`, `internal/acceso/clave_test.go`
- Create: `internal/acceso/sesion.go`, `internal/acceso/sesion_test.go`
- Create: `internal/acceso/limite.go`, `internal/acceso/limite_test.go`

**Interfaces:**
- Produces:
  - `const FicheroClave = "access-key"`
  - `func CargarOGenerar(dir, deEntorno string) (clave string, generada bool, err error)`
  - `type Sesiones struct{...}`; `func NuevasSesiones(clave string, ahora func() time.Time) *Sesiones` (`ahora` nil → `time.Now`); `func (s *Sesiones) ClaveCorrecta(intento string) bool`; `func (s *Sesiones) Emitir() (string, error)`; `func (s *Sesiones) Valida(token string) bool`; `const DuracionSesion = 30 * 24 * time.Hour`.
  - `type Limitador struct{...}`; `func NuevoLimitador(max int, ventana time.Duration, ahora func() time.Time) *Limitador`; `func (l *Limitador) Permitir(clave string) bool`.

- [ ] **Step 1: Tests de la clave**

`internal/acceso/clave_test.go`:

```go
package acceso_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/gdberysan/open-tv/internal/acceso"
)

func TestCargarOGenerarPrefiereLaVariableDeEntorno(t *testing.T) {
	dir := t.TempDir()
	clave, generada, err := acceso.CargarOGenerar(dir, "  desde-el-entorno  ")
	if err != nil {
		t.Fatalf("CargarOGenerar: %v", err)
	}
	if clave != "desde-el-entorno" || generada {
		t.Errorf("clave=%q generada=%v, quiero la del entorno sin espacios y sin generar", clave, generada)
	}
	if _, err := os.Stat(filepath.Join(dir, acceso.FicheroClave)); !os.IsNotExist(err) {
		t.Error("con la clave en el entorno no debe escribirse fichero")
	}
}

func TestCargarOGenerarCreaUnaVezYLuegoReutiliza(t *testing.T) {
	dir := t.TempDir()
	primera, generada, err := acceso.CargarOGenerar(dir, "")
	if err != nil {
		t.Fatalf("primera: %v", err)
	}
	if !generada || len(primera) < 40 {
		t.Fatalf("primera=%q generada=%v: quiero una clave nueva de >= 40 caracteres", primera, generada)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(filepath.Join(dir, acceso.FicheroClave))
		if err != nil {
			t.Fatalf("Stat: %v", err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Errorf("permisos %v, quiero 0600", info.Mode().Perm())
		}
	}

	segunda, generada, err := acceso.CargarOGenerar(dir, "")
	if err != nil {
		t.Fatalf("segunda: %v", err)
	}
	if generada || segunda != primera {
		t.Errorf("segunda=%q generada=%v, quiero la misma clave sin regenerar", segunda, generada)
	}
}

func TestCargarOGenerarRechazaFicheroVacio(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, acceso.FicheroClave), []byte("\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := acceso.CargarOGenerar(dir, ""); err == nil {
		t.Error("un fichero de clave vacío se aceptó: nadie podría entrar")
	}
}
```

- [ ] **Step 2: Verificar que falla**

Run: `go test ./internal/acceso/ -count=1`
Expected: FAIL de compilación (paquete inexistente).

- [ ] **Step 3: Implementar `clave.go`**

```go
// Package acceso protege Open TV cuando escucha fuera de loopback: una clave
// por instalación, sesiones firmadas sin estado y una página de acceso sin
// JavaScript. En loopback no se monta nada de esto.
package acceso

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FicheroClave vive junto a la base de datos, en el mismo directorio de datos.
const FicheroClave = "access-key"

// CargarOGenerar decide la clave de acceso. La variable de entorno gana
// siempre (es lo cómodo en Docker); si no hay, se reutiliza la del fichero; y
// si tampoco, se genera una y se guarda con permisos 0600. generada dice si
// hay que enseñársela a quien arranca, porque no la conoce nadie más.
func CargarOGenerar(dir, deEntorno string) (string, bool, error) {
	if c := strings.TrimSpace(deEntorno); c != "" {
		return c, false, nil
	}
	ruta := filepath.Join(dir, FicheroClave)
	datos, err := os.ReadFile(ruta) // #nosec G304 -- ruta = directorio de datos propio + nombre fijo
	if err == nil {
		c := strings.TrimSpace(string(datos))
		if c == "" {
			return "", false, fmt.Errorf("acceso: %s está vacío; bórralo para generar una clave nueva", ruta)
		}
		return c, false, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", false, fmt.Errorf("acceso: leyendo %s: %w", ruta, err)
	}

	crudo := make([]byte, 32)
	if _, err := rand.Read(crudo); err != nil {
		return "", false, fmt.Errorf("acceso: generando la clave: %w", err)
	}
	c := base64.RawURLEncoding.EncodeToString(crudo)
	if err := os.WriteFile(ruta, []byte(c+"\n"), 0o600); err != nil {
		return "", false, fmt.Errorf("acceso: guardando %s: %w", ruta, err)
	}
	return c, true, nil
}
```

- [ ] **Step 4: Tests de sesiones**

`internal/acceso/sesion_test.go`:

```go
package acceso_test

import (
	"strings"
	"testing"
	"time"

	"github.com/gdberysan/open-tv/internal/acceso"
)

func TestClaveCorrecta(t *testing.T) {
	s := acceso.NuevasSesiones("la-clave", nil)
	if !s.ClaveCorrecta("la-clave") {
		t.Error("la clave buena no entra")
	}
	for _, mala := range []string{"", "la-clav", "la-clave ", "LA-CLAVE"} {
		if s.ClaveCorrecta(mala) {
			t.Errorf("%q entra", mala)
		}
	}
}

func TestSesionEmitidaValidaHastaQueCaduca(t *testing.T) {
	ahora := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	s := acceso.NuevasSesiones("la-clave", func() time.Time { return ahora })

	token, err := s.Emitir()
	if err != nil {
		t.Fatalf("Emitir: %v", err)
	}
	if !s.Valida(token) {
		t.Fatal("el token recién emitido no valida")
	}

	ahora = ahora.Add(acceso.DuracionSesion - time.Minute)
	if !s.Valida(token) {
		t.Error("caducó antes de tiempo")
	}
	ahora = ahora.Add(2 * time.Minute)
	if s.Valida(token) {
		t.Error("sigue valiendo después de caducar")
	}
}

func TestSesionNoSobreviveACambiarLaClave(t *testing.T) {
	token, err := acceso.NuevasSesiones("vieja", nil).Emitir()
	if err != nil {
		t.Fatalf("Emitir: %v", err)
	}
	if acceso.NuevasSesiones("nueva", nil).Valida(token) {
		t.Error("una sesión emitida con la clave vieja vale con la nueva")
	}
}

func TestSesionRechazaTokensManipulados(t *testing.T) {
	s := acceso.NuevasSesiones("la-clave", nil)
	token, err := s.Emitir()
	if err != nil {
		t.Fatalf("Emitir: %v", err)
	}
	partes := strings.Split(token, ".")
	if len(partes) != 4 {
		t.Fatalf("token %q: quiero 4 partes", token)
	}
	lejos := strings.Join([]string{partes[0], "99999999999", partes[2], partes[3]}, ".")
	for _, malo := range []string{"", "v1", "v1.1.2", token + "x", lejos, "v2." + strings.Join(partes[1:], ".")} {
		if s.Valida(malo) {
			t.Errorf("%q valida", malo)
		}
	}
}
```

- [ ] **Step 5: Implementar `sesion.go`**

```go
package acceso

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// DuracionSesion: un televisor o un móvil de casa no deberían pedir la clave
// cada semana.
const DuracionSesion = 30 * 24 * time.Hour

// Sesiones emite y valida tokens sin estado: v1.<caduca>.<nonce>.<mac>. El
// secreto se deriva de la clave, así que cambiarla invalida todas las sesiones
// sin guardar nada en disco.
type Sesiones struct {
	secreto   []byte
	resumen   [32]byte
	ahora     func() time.Time
}

func NuevasSesiones(clave string, ahora func() time.Time) *Sesiones {
	if ahora == nil {
		ahora = time.Now
	}
	mac := hmac.New(sha256.New, []byte(clave))
	_, _ = mac.Write([]byte("open-tv/sesion/v1"))
	return &Sesiones{secreto: mac.Sum(nil), resumen: sha256.Sum256([]byte(clave)), ahora: ahora}
}

// ClaveCorrecta compara resúmenes de longitud fija en tiempo constante: ni el
// contenido ni la longitud de la clave se filtran por el tiempo de respuesta.
func (s *Sesiones) ClaveCorrecta(intento string) bool {
	r := sha256.Sum256([]byte(intento))
	return subtle.ConstantTimeCompare(r[:], s.resumen[:]) == 1
}

func (s *Sesiones) Emitir() (string, error) {
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("acceso: generando el nonce: %w", err)
	}
	caduca := strconv.FormatInt(s.ahora().Add(DuracionSesion).Unix(), 10)
	cuerpo := "v1." + caduca + "." + base64.RawURLEncoding.EncodeToString(nonce)
	return cuerpo + "." + s.firmar(cuerpo), nil
}

func (s *Sesiones) Valida(token string) bool {
	partes := strings.Split(token, ".")
	if len(partes) != 4 || partes[0] != "v1" {
		return false
	}
	cuerpo := strings.Join(partes[:3], ".")
	if !hmac.Equal([]byte(s.firmar(cuerpo)), []byte(partes[3])) {
		return false
	}
	caduca, err := strconv.ParseInt(partes[1], 10, 64)
	if err != nil {
		return false
	}
	return s.ahora().Unix() < caduca
}

func (s *Sesiones) firmar(cuerpo string) string {
	mac := hmac.New(sha256.New, s.secreto)
	_, _ = mac.Write([]byte(cuerpo))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
```

- [ ] **Step 6: Tests del limitador**

`internal/acceso/limite_test.go`:

```go
package acceso_test

import (
	"testing"
	"time"

	"github.com/gdberysan/open-tv/internal/acceso"
)

func TestLimitadorCortaTrasElMaximoYSeRecupera(t *testing.T) {
	ahora := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	l := acceso.NuevoLimitador(3, time.Minute, func() time.Time { return ahora })

	for i := 0; i < 3; i++ {
		if !l.Permitir("10.0.0.2") {
			t.Fatalf("intento %d cortado antes del máximo", i+1)
		}
	}
	if l.Permitir("10.0.0.2") {
		t.Error("el cuarto intento en la misma ventana pasó")
	}
	if !l.Permitir("10.0.0.3") {
		t.Error("otra IP paga los intentos de la primera")
	}

	ahora = ahora.Add(time.Minute + time.Second)
	if !l.Permitir("10.0.0.2") {
		t.Error("no se recupera al pasar la ventana")
	}
}
```

- [ ] **Step 7: Implementar `limite.go`**

```go
package acceso

import (
	"sync"
	"time"
)

// Limitador cuenta intentos de acceso por IP en ventanas fijas. Con una clave
// de 256 bits la fuerza bruta no es viable de todos modos; esto existe para que
// un script no llene el log ni el procesador.
type Limitador struct {
	mu      sync.Mutex
	max     int
	ventana time.Duration
	ahora   func() time.Time
	cuentas map[string]cuenta
}

type cuenta struct {
	desde    time.Time
	intentos int
}

func NuevoLimitador(max int, ventana time.Duration, ahora func() time.Time) *Limitador {
	if ahora == nil {
		ahora = time.Now
	}
	return &Limitador{max: max, ventana: ventana, ahora: ahora, cuentas: make(map[string]cuenta)}
}

func (l *Limitador) Permitir(clave string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	ahora := l.ahora()
	// Limpieza perezosa: el mapa no crece con IPs que ya no intentan nada.
	for k, c := range l.cuentas {
		if ahora.Sub(c.desde) >= l.ventana {
			delete(l.cuentas, k)
		}
	}
	c := l.cuentas[clave]
	if c.intentos == 0 {
		c.desde = ahora
	}
	if c.intentos >= l.max {
		return false
	}
	c.intentos++
	l.cuentas[clave] = c
	return true
}
```

- [ ] **Step 8: Verificar que pasa**

Run: `go test -race -count=1 ./internal/acceso/`
Expected: PASS.

- [ ] **Step 9: Gates y commit**

Gates Go. Luego:

```bash
git add internal/acceso/clave.go internal/acceso/clave_test.go internal/acceso/sesion.go internal/acceso/sesion_test.go internal/acceso/limite.go internal/acceso/limite_test.go
git commit -m "feat(acceso): clave de acceso, sesiones firmadas y limitador

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 4: Middleware de acceso y página `/acceso`

**Files:**
- Create: `internal/acceso/pagina.go`
- Create: `internal/acceso/http.go`
- Create: `internal/acceso/http_test.go`

**Interfaces:**
- Consumes: `Sesiones`, `Limitador` (Tarea 3).
- Produces:
  - `const NombreCookie = "open_tv_sesion"`, `const RutaAcceso = "/acceso"`.
  - `func Exigir(s *Sesiones) func(http.Handler) http.Handler` — exentas `GET /health` y `/acceso` (GET/POST). Sin sesión: navegación HTML (`GET` con `Accept` que contiene `text/html`) → `303` a `/acceso`; resto → `401` con JSON `{"error":"acceso requerido"}`.
  - `func Pagina(s *Sesiones, l *Limitador) http.Handler` — `GET` pinta el formulario; `POST` con campo `clave` crea sesión y redirige `303` a `/`; clave mala → `401` con el formulario y el error; demasiados intentos → `429`.

- [ ] **Step 1: Tests HTTP**

`internal/acceso/http_test.go`:

```go
package acceso_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gdberysan/open-tv/internal/acceso"
)

func montar(t *testing.T) (http.Handler, *acceso.Sesiones) {
	t.Helper()
	s := acceso.NuevasSesiones("la-clave", nil)
	l := acceso.NuevoLimitador(5, time.Minute, nil)
	mux := http.NewServeMux()
	mux.Handle(acceso.RutaAcceso, acceso.Pagina(s, l))
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })
	mux.HandleFunc("/channels", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("[]")) })
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("<html>app</html>")) })
	return acceso.Exigir(s)(mux), s
}

func TestSinSesionLaAPIDa401YLaNavegacionVaAAcceso(t *testing.T) {
	h, _ := montar(t)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/channels", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("/channels sin sesión = %d, quiero 401", rec.Code)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != acceso.RutaAcceso {
		t.Errorf("/ sin sesión = %d → %q, quiero 303 → /acceso", rec.Code, rec.Header().Get("Location"))
	}
}

func TestHealthYAccesoNoExigenSesion(t *testing.T) {
	h, _ := montar(t)
	for _, ruta := range []string{"/health", acceso.RutaAcceso} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, ruta, nil))
		if rec.Code != http.StatusOK {
			t.Errorf("%s = %d, quiero 200", ruta, rec.Code)
		}
	}
}

func postClave(h http.Handler, clave, ip string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, acceso.RutaAcceso, strings.NewReader(url.Values{"clave": {clave}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.RemoteAddr = ip + ":5555"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestClaveBuenaCreaSesionYAbreLaAPI(t *testing.T) {
	h, _ := montar(t)

	rec := postClave(h, "la-clave", "10.0.0.2")
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/" {
		t.Fatalf("POST clave buena = %d → %q, quiero 303 → /", rec.Code, rec.Header().Get("Location"))
	}
	var cookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == acceso.NombreCookie {
			cookie = c
		}
	}
	if cookie == nil {
		t.Fatal("no hay cookie de sesión")
	}
	if !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.Path != "/" {
		t.Errorf("cookie %+v: quiero HttpOnly, SameSite=Strict y Path=/", cookie)
	}
	if cookie.Secure {
		t.Error("Secure en una petición http sin X-Forwarded-Proto: el navegador no la devolvería")
	}

	req := httptest.NewRequest(http.MethodGet, "/channels", nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("/channels con sesión = %d, quiero 200", rec.Code)
	}
}

func TestClaveBuenaDetrasDeHTTPSMarcaLaCookieSecure(t *testing.T) {
	h, _ := montar(t)
	req := httptest.NewRequest(http.MethodPost, acceso.RutaAcceso, strings.NewReader("clave=la-clave"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-Forwarded-Proto", "https")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	for _, c := range rec.Result().Cookies() {
		if c.Name == acceso.NombreCookie && !c.Secure {
			t.Error("detrás de HTTPS la cookie no lleva Secure")
		}
	}
}

func TestClaveMalaNoCreaSesion(t *testing.T) {
	h, _ := montar(t)
	rec := postClave(h, "otra", "10.0.0.2")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("POST clave mala = %d, quiero 401", rec.Code)
	}
	if len(rec.Result().Cookies()) != 0 {
		t.Error("una clave mala dejó cookie")
	}
	if !strings.Contains(rec.Body.String(), `role="alert"`) {
		t.Error("el error no se anuncia (falta role=alert)")
	}
}

func TestDemasiadosIntentosDan429(t *testing.T) {
	h, _ := montar(t)
	for i := 0; i < 5; i++ {
		postClave(h, "otra", "10.0.0.9")
	}
	if rec := postClave(h, "la-clave", "10.0.0.9"); rec.Code != http.StatusTooManyRequests {
		t.Errorf("sexto intento = %d, quiero 429 aunque la clave sea buena", rec.Code)
	}
}

func TestPaginaDeAccesoEnInglesYConCSPSinScripts(t *testing.T) {
	h, _ := montar(t)
	req := httptest.NewRequest(http.MethodGet, acceso.RutaAcceso, nil)
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	cuerpo := rec.Body.String()
	if !strings.Contains(cuerpo, `lang="en"`) || !strings.Contains(cuerpo, "Access key") {
		t.Errorf("la página no sale en inglés:\n%s", cuerpo)
	}
	if strings.Contains(cuerpo, "<script") {
		t.Error("la página de acceso lleva scripts")
	}
	csp := rec.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "default-src 'none'") || !strings.Contains(csp, "form-action 'self'") {
		t.Errorf("CSP %q", csp)
	}
	if !strings.Contains(cuerpo, `<label for="clave"`) || !strings.Contains(cuerpo, `autocomplete="current-password"`) {
		t.Error("el campo no tiene etiqueta asociada o autocompletado de contraseña")
	}
}

func TestPaginaDeAccesoPorDefectoEnEspanol(t *testing.T) {
	h, _ := montar(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, acceso.RutaAcceso, nil))
	if !strings.Contains(rec.Body.String(), `lang="es"`) || !strings.Contains(rec.Body.String(), "Clave de acceso") {
		t.Errorf("sin Accept-Language no sale en español:\n%s", rec.Body.String())
	}
}
```

- [ ] **Step 2: Verificar que falla**

Run: `go test ./internal/acceso/ -count=1`
Expected: FAIL de compilación (`undefined: acceso.Exigir`).

- [ ] **Step 3: Implementar `pagina.go`**

```go
package acceso

import (
	"html/template"
	"net/http"
	"strings"
)

type textos struct {
	Lang, Titulo, Explicacion, Etiqueta, Boton, Error, Ayuda string
}

var textosPorIdioma = map[string]textos{
	"es": {
		Lang:        "es",
		Titulo:      "Korven Open TV",
		Explicacion: "Este Open TV es accesible desde la red, así que pide la clave de acceso.",
		Etiqueta:    "Clave de acceso",
		Boton:       "Entrar",
		Error:       "La clave no es correcta.",
		Ayuda:       "La clave aparece en el registro del servidor al arrancar por primera vez, o con: open-tv access-key",
	},
	"en": {
		Lang:        "en",
		Titulo:      "Korven Open TV",
		Explicacion: "This Open TV is reachable from the network, so it asks for the access key.",
		Etiqueta:    "Access key",
		Boton:       "Sign in",
		Error:       "That key isn't correct.",
		Ayuda:       "The key is printed in the server log on first start, or run: open-tv access-key",
	},
}

// idiomaDe elige es o en por Accept-Language. Español por defecto, como el
// resto de la app.
func idiomaDe(r *http.Request) textos {
	for _, parte := range strings.Split(r.Header.Get("Accept-Language"), ",") {
		etiqueta := strings.ToLower(strings.TrimSpace(strings.SplitN(parte, ";", 2)[0]))
		if strings.HasPrefix(etiqueta, "es") {
			return textosPorIdioma["es"]
		}
		if strings.HasPrefix(etiqueta, "en") {
			return textosPorIdioma["en"]
		}
	}
	return textosPorIdioma["es"]
}

// Los colores son los tokens de marca de web/src/estilos/tokens/colors.css
// copiados a mano: esta página se sirve ANTES de la sesión, sin acceso a los
// assets del cliente.
var plantilla = template.Must(template.New("acceso").Parse(`<!doctype html>
<html lang="{{.T.Lang}}">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.T.Titulo}}</title>
<style>
:root{color-scheme:dark;--grafito:#0E131B;--carbon:#171E29;--ambar:#FF8A2B;--acero:#97A3B2;--hueso:#EFF3F8;--error:#E5604D}
*{box-sizing:border-box}
body{margin:0;min-height:100vh;display:grid;place-items:center;background:var(--grafito);color:var(--hueso);font:16px/1.5 system-ui,-apple-system,"Segoe UI",sans-serif;padding:16px}
main{width:100%;max-width:380px;background:var(--carbon);border:1px solid #283142;border-radius:16px;padding:28px}
h1{margin:0 0 8px;font-size:22px}h1 span{color:var(--ambar)}
p{margin:0 0 20px;color:var(--acero)}
label{display:block;font-weight:600;margin-bottom:6px}
input{width:100%;padding:10px 12px;border-radius:10px;border:1px solid #3a4658;background:var(--grafito);color:var(--hueso);font:inherit}
input:focus-visible,button:focus-visible{outline:3px solid var(--ambar);outline-offset:2px}
button{margin-top:16px;width:100%;padding:10px;border:0;border-radius:10px;background:var(--ambar);color:var(--grafito);font:inherit;font-weight:700;cursor:pointer}
.error{color:var(--error);margin:12px 0 0}
small{display:block;margin-top:20px;color:var(--acero)}
</style>
</head>
<body>
<main>
<h1>KORVEN <span>OPEN TV</span></h1>
<p>{{.T.Explicacion}}</p>
<form method="post" action="/acceso">
<label for="clave">{{.T.Etiqueta}}</label>
<input id="clave" name="clave" type="password" autocomplete="current-password" required autofocus{{if .ConError}} aria-invalid="true" aria-describedby="error"{{end}}>
{{if .ConError}}<p id="error" class="error" role="alert">{{.T.Error}}</p>{{end}}
<button type="submit">{{.T.Boton}}</button>
</form>
<small>{{.T.Ayuda}}</small>
</main>
</body>
</html>
`))

const cspPagina = "default-src 'none'; style-src 'unsafe-inline'; form-action 'self'; frame-ancestors 'none'; base-uri 'none'"

func pintar(w http.ResponseWriter, r *http.Request, estado int, conError bool) {
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Content-Security-Policy", cspPagina)
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("X-Frame-Options", "DENY")
	h.Set("Cache-Control", "no-store")
	w.WriteHeader(estado)
	_ = plantilla.Execute(w, struct {
		T        textos
		ConError bool
	}{idiomaDe(r), conError})
}
```

- [ ] **Step 4: Implementar `http.go`**

```go
package acceso

import (
	"net"
	"net/http"
	"strings"
)

// NombreCookie y RutaAcceso son públicos para que el router y las pruebas no
// repitan literales.
const (
	NombreCookie = "open_tv_sesion"
	RutaAcceso   = "/acceso"
)

// maxCuerpoAcceso: un formulario con un campo no necesita más.
const maxCuerpoAcceso = 4 << 10

// Exigir deja pasar solo peticiones con sesión válida, salvo /health (lo usan
// el HEALTHCHECK de Docker y la detección de instancia viva) y la propia
// página de acceso.
func Exigir(s *Sesiones) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if exenta(r) || conSesion(s, r) {
				next.ServeHTTP(w, r)
				return
			}
			if r.Method == http.MethodGet && strings.Contains(r.Header.Get("Accept"), "text/html") {
				http.Redirect(w, r, RutaAcceso, http.StatusSeeOther)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"acceso requerido"}`))
		})
	}
}

func exenta(r *http.Request) bool {
	if r.URL.Path == RutaAcceso {
		return true
	}
	return r.URL.Path == "/health" && (r.Method == http.MethodGet || r.Method == http.MethodHead)
}

func conSesion(s *Sesiones, r *http.Request) bool {
	c, err := r.Cookie(NombreCookie)
	return err == nil && s.Valida(c.Value)
}

// Pagina sirve GET y POST de /acceso.
func Pagina(s *Sesiones, l *Limitador) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead:
			if conSesion(s, r) {
				http.Redirect(w, r, "/", http.StatusSeeOther)
				return
			}
			pintar(w, r, http.StatusOK, false)
		case http.MethodPost:
			entrar(s, l, w, r)
		default:
			w.Header().Set("Allow", "GET, POST")
			http.Error(w, "método no permitido", http.StatusMethodNotAllowed)
		}
	})
}

func entrar(s *Sesiones, l *Limitador, w http.ResponseWriter, r *http.Request) {
	if !l.Permitir(ipDe(r)) {
		http.Error(w, "demasiados intentos; espera un minuto", http.StatusTooManyRequests)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxCuerpoAcceso)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "formulario inválido", http.StatusBadRequest)
		return
	}
	if !s.ClaveCorrecta(r.PostForm.Get("clave")) {
		pintar(w, r, http.StatusUnauthorized, true)
		return
	}
	token, err := s.Emitir()
	if err != nil {
		http.Error(w, "no se pudo crear la sesión", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     NombreCookie,
		Value:    token,
		Path:     "/",
		MaxAge:   int(DuracionSesion.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		// Detrás de un reverse proxy con TLS la petición llega en http: se mira
		// X-Forwarded-Proto. Falsificarlo solo sirve para que el propio
		// navegador del atacante no devuelva la cookie.
		Secure: r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https"),
	})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// ipDe usa RemoteAddr y no X-Forwarded-For: detrás de un reverse proxy todos
// comparten IP y el límite es global, que es peor para el uso pero no abre
// la puerta a saltárselo falsificando una cabecera.
func ipDe(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
```

- [ ] **Step 5: Verificar que pasa**

Run: `go test -race -count=1 ./internal/acceso/`
Expected: PASS.

- [ ] **Step 6: Gates y commit**

Gates Go. Luego:

```bash
git add internal/acceso/pagina.go internal/acceso/http.go internal/acceso/http_test.go
git commit -m "feat(acceso): middleware de sesión y página de acceso sin JavaScript

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 5: Modo red en el router y en el arranque

**Files:**
- Modify: `internal/api/middleware/mismo_origen.go`, `internal/api/middleware/mismo_origen_test.go`
- Modify: `internal/api/router.go`, `internal/api/router_test.go`
- Modify: `cmd/open-tv/main.go`, `cmd/open-tv/main_test.go`

**Interfaces:**
- Consumes: `acceso.Exigir`, `acceso.Pagina`, `acceso.NuevasSesiones`, `acceso.NuevoLimitador`, `acceso.CargarOGenerar`, `acceso.RutaAcceso` (Tareas 3–4); `api.Options.Catalogo`, `api.Options.Firmador` (Tarea 2).
- Produces:
  - `api.Options.Sesiones *acceso.Sesiones` (nil = loopback, sin autenticación) y `api.Options.Limitador *acceso.Limitador` (nil con Sesiones → el router crea `NuevoLimitador(5, time.Minute, nil)`).
  - En `main.go`: `func modoRed(ln net.Listener) bool` (= `!esLoopback(ln)`).

- [ ] **Step 1: Test de `Origin` en MismoOrigen**

Añadir a `internal/api/middleware/mismo_origen_test.go`:

```go
// Sin Sec-Fetch-Site (navegadores viejos, o un cliente que no lo manda), un
// Origin de otro sitio en un método mutante también se corta.
func TestMismoOrigenCortaOriginAjenoEnMutantes(t *testing.T) {
	h := middleware.MismoOrigen(nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	for _, c := range []struct {
		origin string
		quiero int
	}{
		{"http://atacante.example", http.StatusForbidden},
		{"http://tv.local:8080", http.StatusNoContent},
		{"", http.StatusNoContent},
	} {
		req := httptest.NewRequest(http.MethodPost, "http://tv.local:8080/sources", nil)
		if c.origin != "" {
			req.Header.Set("Origin", c.origin)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != c.quiero {
			t.Errorf("Origin %q = %d, quiero %d", c.origin, rec.Code, c.quiero)
		}
	}
}
```

- [ ] **Step 2: Verificar que falla**

Run: `go test ./internal/api/middleware/ -run TestMismoOrigenCortaOriginAjenoEnMutantes -count=1`
Expected: FAIL (el caso `atacante.example` da 204).

- [ ] **Step 3: Implementar**

En `mismo_origen.go`, dentro del bloque `if r.Method != http.MethodGet && r.Method != http.MethodHead {`, después de la comprobación de `Sec-Fetch-Site`:

```go
				// Origin cubre a los clientes que no mandan Sec-Fetch-Site. En
				// modo red la lista de Host está vacía, así que esta es la
				// comprobación que queda contra CSRF (con SameSite=Strict).
				if origen := r.Header.Get("Origin"); origen != "" {
					u, err := url.Parse(origen)
					if err != nil || u.Host != r.Host {
						http.Error(w, "origen cruzado no permitido", http.StatusForbidden)
						return
					}
				}
```

Cambiar el import a `import ("net/http"; "net/url")` con formato gofmt.

- [ ] **Step 4: Verificar que pasa**

Run: `go test -race -count=1 ./internal/api/middleware/`
Expected: PASS.

- [ ] **Step 5: Tests del router en modo red**

Añadir a `internal/api/router_test.go`:

```go
func routerModoRed(t *testing.T) http.Handler {
	t.Helper()
	return api.NewRouter(slog.New(slog.DiscardHandler), repoVacio{}, provVacio{}, streamsVacio{}, nil, syncVacio{}, sourcesVacio{}, t.TempDir(), nil,
		api.Options{ProxyActivo: true, Sesiones: acceso.NuevasSesiones("la-clave", nil)})
}

func TestModoRedExigeSesionEnAPIYProxy(t *testing.T) {
	r := routerModoRed(t)
	for _, c := range []struct {
		ruta   string
		quiero int
	}{
		{"/channels", http.StatusUnauthorized},
		{"/sources", http.StatusUnauthorized},
		{"/proxy/hls?u=x", http.StatusUnauthorized},
		{"/acceso", http.StatusOK},
	} {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, c.ruta, nil))
		if rec.Code != c.quiero {
			t.Errorf("%s = %d, quiero %d", c.ruta, rec.Code, c.quiero)
		}
	}
}

func TestLoopbackNoMontaAcceso(t *testing.T) {
	r := api.NewRouter(slog.New(slog.DiscardHandler), repoVacio{}, provVacio{}, streamsVacio{}, nil, syncVacio{}, sourcesVacio{}, t.TempDir(), nil, api.Options{})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/sources", nil))
	if rec.Code == http.StatusUnauthorized {
		t.Error("sin Sesiones el router exige acceso: el modo loopback cambió")
	}
}
```

Añadir `"github.com/gdberysan/open-tv/internal/acceso"` a los imports. El `/health` exento ya lo cubre `TestHealthYAccesoNoExigenSesion` de la Tarea 4, y aquí `sqlDB` es nil, así que no se pide.

- [ ] **Step 6: Verificar que falla**

Run: `go test ./internal/api/ -run 'ModoRed|LoopbackNoMontaAcceso' -count=1`
Expected: FAIL de compilación (`unknown field Sesiones`).

- [ ] **Step 7: Implementar en el router**

En `api.Options`:

```go
	// Sesiones activa el modo red: todo salvo /health y /acceso exige sesión.
	// nil es el modo loopback de siempre, sin autenticación.
	Sesiones *acceso.Sesiones
	// Limitador de intentos de /acceso. nil con Sesiones: 5 por minuto por IP.
	Limitador *acceso.Limitador
```

En `NewRouter`, justo después de `r.Use(middleware.MismoOrigen(opts.HostsPermitidos))`:

```go
	// Modo red: la sesión va antes que el logger y el limitador de
	// concurrencia, para que una avalancha sin sesión no ocupe huecos.
	if opts.Sesiones != nil {
		r.Use(acceso.Exigir(opts.Sesiones))
		limitador := opts.Limitador
		if limitador == nil {
			limitador = acceso.NuevoLimitador(5, time.Minute, nil)
		}
		pagina := acceso.Pagina(opts.Sesiones, limitador)
		r.Get(acceso.RutaAcceso, pagina.ServeHTTP)
		r.Post(acceso.RutaAcceso, pagina.ServeHTTP)
	}
```

Importar `"github.com/gdberysan/open-tv/internal/acceso"`.

- [ ] **Step 8: Verificar que pasa**

Run: `go test -race -count=1 ./internal/api/...`
Expected: PASS.

- [ ] **Step 9: Test de arranque en modo red**

Añadir a `cmd/open-tv/main_test.go` (añadir `"net/url"` a imports si falta):

```go
// Escuchando fuera de loopback, run() entra en modo red: /health responde sin
// sesión, la API pide la clave, y con la clave correcta la API abre.
func TestRunEnModoRedExigeLaClave(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DB_PATH", filepath.Join(dir, "test.db"))
	t.Setenv("LISTEN_ADDR", "0.0.0.0:18087")
	t.Setenv("OPEN_TV_ACCESS_KEY", "clave-de-prueba")

	ctx, cancel := context.WithCancel(context.Background())
	errc := make(chan error, 1)
	go func() { errc <- run(ctx, slog.New(slog.DiscardHandler), true) }()
	defer func() {
		cancel()
		select {
		case <-errc:
		case <-time.After(20 * time.Second):
			t.Error("run no terminó tras cancelar")
		}
	}()

	base := "http://127.0.0.1:18087"
	var err error
	for i := 0; i < 60; i++ {
		var resp *http.Response
		resp, err = http.Get(base + "/health")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("/health = %d, quiero 200 sin sesión", resp.StatusCode)
			}
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("no llegó a escuchar: %v", err)
	}

	resp, err := http.Get(base + "/sources")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("/sources sin sesión = %d, quiero 401", resp.StatusCode)
	}

	cliente := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err = cliente.PostForm(base+"/acceso", url.Values{"clave": {"clave-de-prueba"}})
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("POST /acceso = %d, quiero 303", resp.StatusCode)
	}
	req, _ := http.NewRequest(http.MethodGet, base+"/sources", nil)
	for _, c := range resp.Cookies() {
		req.AddCookie(c)
	}
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("/sources con sesión = %d, quiero 200", resp.StatusCode)
	}

	if _, err := os.Stat(filepath.Join(dir, "access-key")); !os.IsNotExist(err) {
		t.Error("con OPEN_TV_ACCESS_KEY no debe escribirse fichero de clave")
	}
}
```

- [ ] **Step 10: Verificar que falla**

Run: `go test ./cmd/open-tv/ -run TestRunEnModoRedExigeLaClave -count=1`
Expected: FAIL (`/sources sin sesión = 403` o `200`, no 401: el modo red no existe).

- [ ] **Step 11: Implementar en `main.go`**

Import `"github.com/gdberysan/open-tv/internal/acceso"` y `"github.com/gdberysan/open-tv/internal/proxy"`.

Tras resolver `fuentesDir` (paso 3), añadir:

```go
	// 3b. Modo red. Lo decide la IP real del listener, no LISTEN_ADDR (ver
	// esLoopback). Fuera de loopback la API y el proxy quedan al alcance de la
	// red, así que se exige clave; la clave vive junto a la base de datos.
	var sesiones *acceso.Sesiones
	enRed := modoRed(ln)
	if enRed {
		clave, generada, err := acceso.CargarOGenerar(filepath.Dir(dbPath), os.Getenv("OPEN_TV_ACCESS_KEY"))
		if err != nil {
			_ = ln.Close()
			return fmt.Errorf("preparando la clave de acceso: %w", err)
		}
		sesiones = acceso.NuevasSesiones(clave, nil)
		if generada {
			// Única vez que la clave sale por el log: nadie más la conoce.
			logger.Warn("Modo red: abre Open TV desde cualquier dispositivo con esta clave de acceso",
				slog.String("clave", clave),
				slog.String("guardada_en", filepath.Join(filepath.Dir(dbPath), acceso.FicheroClave)))
		} else {
			logger.Info("Modo red: se exige la clave de acceso (open-tv access-key la muestra)")
		}
	}

	firmador, err := proxy.NuevoFirmadorAleatorio()
	if err != nil {
		_ = ln.Close()
		return fmt.Errorf("preparando la firma del proxy: %w", err)
	}
```

En `api.Options{...}`, sustituir `ProxyActivo: esLoopback(ln),` y `HostsPermitidos: hostsPermitidos(ln),` por:

```go
			// El proxy se monta siempre: solo relaya URLs del catálogo o
			// firmadas, y en modo red además exige sesión.
			ProxyActivo:     true,
			Firmador:        firmador,
			Sesiones:        sesiones,
			HostsPermitidos: hostsPermitidos(ln),
```

Cambiar `hostsPermitidos` para que devuelva `nil` en modo red, y añadir `modoRed`:

```go
// modoRed: cualquier listener que no sea loopback (0.0.0.0, una IP de la LAN)
// deja la API al alcance de otras máquinas.
func modoRed(ln net.Listener) bool {
	return !esLoopback(ln)
}

// hostsPermitidos construye la lista blanca de Host para el middleware
// anti-rebinding (Ruling R13) a partir del puerto REAL con el que se acabó
// enlazando (no LISTEN_ADDR: pudo haber saltado a un puerto de fallback). Los
// tres literales de loopback son los que un navegador puede usar para llegar
// aquí; un dominio atacante que resuelve a 127.0.0.1 llega con su propio
// Host y no está en esta lista.
//
// En modo red no hay lista: el Host es cualquiera con el que se llegue a la
// máquina (IP de la LAN, nombre local, dominio del reverse proxy), y el
// control de acceso lo hace la sesión. El DNS-rebinding no sirve contra ella:
// el dominio atacante no tiene la cookie.
func hostsPermitidos(ln net.Listener) []string {
	if modoRed(ln) {
		return nil
	}
	addr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		return nil
	}
	puerto := fmt.Sprintf("%d", addr.Port)
	return []string{
		"127.0.0.1:" + puerto,
		"localhost:" + puerto,
		"[::1]:" + puerto,
	}
}
```

Actualizar el comentario de `esLoopback`: «esLoopback decide el modo: loopback (sin autenticación) o red (con clave).»

Actualizar el comentario del paso 1 («Loopback por defecto: la API no tiene auth…») añadiendo al final: «Fuera de loopback entra el modo red (paso 3b), que exige clave.»

En modo red no se abre el navegador (la URL 0.0.0.0 no sirve y suele ser un servidor sin pantalla). Sustituir `if !sinNavegador {` (el segundo, tras arrancar el servidor) por `if !sinNavegador && !enRed {`.

- [ ] **Step 12: Verificar que pasa**

Run: `go test -race -count=1 ./cmd/open-tv/ ./internal/api/...`
Expected: PASS.

- [ ] **Step 13: Gates y commit**

Gates Go. Luego:

```bash
git add internal/api/middleware/mismo_origen.go internal/api/middleware/mismo_origen_test.go internal/api/router.go internal/api/router_test.go cmd/open-tv/main.go cmd/open-tv/main_test.go
git commit -m "feat: modo red con clave de acceso fuera de loopback

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 6: Subcomandos `access-key` y `healthcheck`

**Files:**
- Create: `cmd/open-tv/subcomandos.go`
- Create: `cmd/open-tv/subcomandos_test.go`
- Modify: `cmd/open-tv/main.go` (`manejaMetaComando`)

**Interfaces:**
- Consumes: `acceso.CargarOGenerar`, `acceso.FicheroClave`; `datadir.RutaDB()`; `InstanciaViva(ctx, base)`.
- Produces:
  - `func mostrarClave(w io.Writer) int` — imprime la clave (`OPEN_TV_ACCESS_KEY` o fichero). Si no hay fichero NO genera: exit 1 con mensaje.
  - `func healthcheck(ctx context.Context, listenAddr string) int` — 0 si `http://127.0.0.1:<puerto>/health` responde 200 con `web_ui`; 1 si no.
  - `func baseLocal(listenAddr string) string`.

- [ ] **Step 1: Tests**

`cmd/open-tv/subcomandos_test.go`:

```go
package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBaseLocalSiempreApuntaALoopback(t *testing.T) {
	for entrada, quiero := range map[string]string{
		"":                "http://127.0.0.1:8080",
		"0.0.0.0:9000":    "http://127.0.0.1:9000",
		"127.0.0.1:8081":  "http://127.0.0.1:8081",
		"192.168.1.5:80":  "http://127.0.0.1:80",
		"[::]:7000":       "http://127.0.0.1:7000",
	} {
		if got := baseLocal(entrada); got != quiero {
			t.Errorf("baseLocal(%q) = %q, quiero %q", entrada, got, quiero)
		}
	}
}

func TestHealthcheck(t *testing.T) {
	sano := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"status":"ok","web_ui":true}`))
	}))
	defer sano.Close()
	if code := healthcheck(context.Background(), strings.TrimPrefix(sano.URL, "http://")); code != 0 {
		t.Errorf("servidor sano → %d, quiero 0", code)
	}
	if code := healthcheck(context.Background(), "127.0.0.1:1"); code != 1 {
		t.Errorf("nada escuchando → %d, quiero 1", code)
	}
}

func TestMostrarClaveLeeEntornoOFicheroSinGenerar(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DB_PATH", filepath.Join(dir, "open-tv.db"))
	t.Setenv("OPEN_TV_ACCESS_KEY", "")

	var out bytes.Buffer
	if code := mostrarClave(&out); code != 1 {
		t.Errorf("sin clave → %d, quiero 1", code)
	}
	if _, err := os.Stat(filepath.Join(dir, "access-key")); !os.IsNotExist(err) {
		t.Error("mostrarClave generó una clave: solo debe leer")
	}

	if err := os.WriteFile(filepath.Join(dir, "access-key"), []byte("del-fichero\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if code := mostrarClave(&out); code != 0 || strings.TrimSpace(out.String()) != "del-fichero" {
		t.Errorf("fichero → code %d, salida %q", code, out.String())
	}

	t.Setenv("OPEN_TV_ACCESS_KEY", "del-entorno")
	out.Reset()
	if code := mostrarClave(&out); code != 0 || strings.TrimSpace(out.String()) != "del-entorno" {
		t.Errorf("entorno → code %d, salida %q", code, out.String())
	}
}

func TestMetaComandosNuevosNoArrancanServidor(t *testing.T) {
	var out bytes.Buffer
	t.Setenv("OPEN_TV_ACCESS_KEY", "x")
	if manejado, _ := manejaMetaComando([]string{"access-key"}, &out); !manejado {
		t.Error("access-key no se reconoce como meta-comando")
	}
	t.Setenv("LISTEN_ADDR", "127.0.0.1:1")
	if manejado, code := manejaMetaComando([]string{"healthcheck"}, &out); !manejado || code != 1 {
		t.Errorf("healthcheck → manejado=%v code=%d, quiero true/1", manejado, code)
	}
}
```

- [ ] **Step 2: Verificar que falla**

Run: `go test ./cmd/open-tv/ -run 'BaseLocal|Healthcheck|MostrarClave|MetaComandosNuevos' -count=1`
Expected: FAIL de compilación (`undefined: baseLocal`).

- [ ] **Step 3: Implementar `subcomandos.go`**

```go
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/gdberysan/open-tv/internal/acceso"
	"github.com/gdberysan/open-tv/internal/datadir"
)

// mostrarClave imprime la clave de acceso del modo red. Solo lee: generar una
// clave desde aquí dejaría una que el servidor en marcha no conoce.
func mostrarClave(w io.Writer) int {
	if c := strings.TrimSpace(os.Getenv("OPEN_TV_ACCESS_KEY")); c != "" {
		fmt.Fprintln(w, c)
		return 0
	}
	dbPath, err := datadir.RutaDB()
	if err != nil {
		fmt.Fprintf(w, "no se pudo resolver el directorio de datos: %v\n", err)
		return 1
	}
	ruta := filepath.Join(filepath.Dir(dbPath), acceso.FicheroClave)
	datos, err := os.ReadFile(ruta) // #nosec G304 -- directorio de datos propio + nombre fijo
	if errors.Is(err, os.ErrNotExist) {
		fmt.Fprintf(w, "todavía no hay clave: se crea al arrancar Open TV fuera de loopback (%s)\n", ruta)
		return 1
	}
	if err != nil {
		fmt.Fprintf(w, "no se pudo leer %s: %v\n", ruta, err)
		return 1
	}
	fmt.Fprintln(w, strings.TrimSpace(string(datos)))
	return 0
}

// healthcheck es el HEALTHCHECK de la imagen: distroless no trae curl.
func healthcheck(ctx context.Context, listenAddr string) int {
	if InstanciaViva(ctx, baseLocal(listenAddr)) {
		return 0
	}
	return 1
}

// baseLocal traduce LISTEN_ADDR a una URL de loopback con el mismo puerto: el
// healthcheck corre dentro del propio contenedor o máquina.
func baseLocal(listenAddr string) string {
	puerto := "8080"
	if listenAddr != "" {
		if _, p, err := net.SplitHostPort(listenAddr); err == nil && p != "" {
			puerto = p
		}
	}
	return "http://127.0.0.1:" + puerto
}
```

- [ ] **Step 4: Conectar en `manejaMetaComando`**

En `main.go`, añadir casos al `switch args[0]`:

```go
	case "access-key":
		return true, mostrarClave(w)
	case "healthcheck":
		return true, healthcheck(context.Background(), os.Getenv("LISTEN_ADDR"))
```

Y ampliar su comentario: «…hoy, la versión, la clave de acceso del modo red y el healthcheck de Docker.»

- [ ] **Step 5: Verificar que pasa**

Run: `go test -race -count=1 ./cmd/open-tv/`
Expected: PASS.

- [ ] **Step 6: Gates y commit**

Gates Go. Luego:

```bash
git add cmd/open-tv/subcomandos.go cmd/open-tv/subcomandos_test.go cmd/open-tv/main.go
git commit -m "feat(cli): subcomandos access-key y healthcheck

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 7: El cliente web vuelve a `/acceso` si la sesión caduca

**Files:**
- Modify: `web/src/datos/http.ts`
- Test: `web/src/datos/http.test.ts`

**Interfaces:**
- Produces: `pedir()` ante un 401 llama a `location.assign('/acceso')` y lanza `Error('sesión caducada')`.

- [ ] **Step 1: Test**

Añadir dentro del `describe('HttpCatalog', …)` de `web/src/datos/http.test.ts`:

```ts
  // Modo red: si la sesión caduca con la app abierta, la API responde 401 y
  // lo correcto es volver a la página de acceso, no pintar "gateway caído".
  it('un 401 lleva a la página de acceso', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => new Response('{"error":"acceso requerido"}', { status: 401 })))
    const assign = vi.fn()
    vi.stubGlobal('location', { assign, protocol: 'http:' })

    const c = crearHttpCatalog('')
    await expect(c.fuentes()).rejects.toThrow('sesión caducada')
    expect(assign).toHaveBeenCalledWith('/acceso')
  })
```

- [ ] **Step 2: Verificar que falla**

Run: `cd web && npx vitest run src/datos/http.test.ts`
Expected: FAIL (lanza `respuesta 401`, `assign` no llamado).

- [ ] **Step 3: Implementar**

En `pedir()` de `web/src/datos/http.ts`, antes de `if (!resp.ok) throw …`:

```ts
  // Modo red (el servidor escucha fuera de loopback y exige clave): un 401 es
  // una sesión caducada o revocada. La página de acceso es HTML del servidor,
  // fuera de la SPA, así que se navega a ella.
  if (resp.status === 401) {
    location.assign('/acceso')
    throw new Error('sesión caducada')
  }
```

- [ ] **Step 4: Verificar que pasa y gates web**

Run: `cd web && npm run check && npm test && npm run build; cd .. && git checkout -- internal/ui/dist/.gitkeep`
Expected: todo PASS. Bundle propio sigue ≤ 80 KB gzip.

- [ ] **Step 5: Commit**

```bash
git add web/src/datos/http.ts web/src/datos/http.test.ts
git commit -m "feat(web): una sesión caducada vuelve a la página de acceso

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 8: e2e del modo red

**Files:**
- Modify: `web/tests/e2e/global-setup.ts`
- Create: `web/tests/e2e/modo-red.spec.ts`

**Interfaces:**
- Consumes: todo lo anterior, a través del binario real.
- Produces: `export const PUERTO_RED = 8091` y `export const CLAVE_E2E = 'clave-e2e-no-secreta'` en `global-setup.ts`.

- [ ] **Step 1: Segunda instancia en modo red**

En `web/tests/e2e/global-setup.ts`:

Debajo de `export const PUERTO_APP = 8090` añadir:

```ts
/** Segunda instancia del MISMO binario, escuchando en 0.0.0.0: modo red con
 *  clave. Comparte el servidor de fixtures, con su propia base de datos. */
export const PUERTO_RED = 8091
export const CLAVE_E2E = 'clave-e2e-no-secreta'
```

Debajo de `let binario: ChildProcess | undefined` añadir `let binarioRed: ChildProcess | undefined`.

Tras `await esperarSync(\`http://127.0.0.1:${PUERTO_APP}\`)` y ANTES del margen de 3 s, añadir:

```ts
  const datosRed = mkdtempSync(join(tmpdir(), 'opentv-e2e-red-'))
  binarioRed = spawn(join(RAIZ, 'open-tv'), ['serve', '--no-browser'], {
    cwd: RAIZ,
    env: {
      ...process.env,
      DB_PATH: join(datosRed, 'e2e.db'),
      LISTEN_ADDR: `0.0.0.0:${PUERTO_RED}`,
      OPEN_TV_ACCESS_KEY: CLAVE_E2E,
      IPTV_ORG_URL: `http://127.0.0.1:${PUERTO_FIXTURES}/cors/catalogo.m3u`,
      HEALTH_INTERVAL: '10s',
      OPEN_TV_PERMITIR_DESTINOS_PRIVADOS: '1',
    },
    stdio: 'inherit',
  })
  // /health no exige sesión, así que esperarSync funciona igual en modo red.
  await esperarSync(`http://127.0.0.1:${PUERTO_RED}`)
```

En el teardown devuelto, añadir `binarioRed?.kill('SIGTERM')` junto a `binario?.kill('SIGTERM')`.

- [ ] **Step 2: La spec**

`web/tests/e2e/modo-red.spec.ts`:

```ts
import { expect, test } from '@playwright/test'
import { CLAVE_E2E, PUERTO_RED } from './global-setup'

test.skip(
  ({ browserName }) => browserName === 'webkit' && !!process.env.CI,
  'WebKit de Linux no garantiza H.264; el gate de Safari es manual',
)

const BASE = `http://127.0.0.1:${PUERTO_RED}`

test('en modo red la app pide la clave, rechaza una mala y reproduce tras entrar', async ({ page }) => {
  await page.goto(BASE + '/')
  await expect(page).toHaveURL(BASE + '/acceso')

  const campo = page.getByLabel('Clave de acceso')
  await campo.fill('clave-equivocada')
  await page.getByRole('button', { name: 'Entrar' }).click()
  await expect(page.getByRole('alert')).toHaveText('La clave no es correcta.')

  await page.getByLabel('Clave de acceso').fill(CLAVE_E2E)
  await page.getByRole('button', { name: 'Entrar' }).click()
  await expect(page).toHaveURL(BASE + '/')

  // Mismo canal que reproduccion.spec.ts: en Chromium y Firefox solo se ve por
  // el proxy, así que esto prueba sesión + proxy + URLs firmadas juntos.
  await page.locator('.lista-lateral article', { hasText: 'Canal Con CORS' }).locator('button.abrir').click()
  const video = page.locator('video')
  await expect
    .poll(async () => video.evaluate((v: HTMLVideoElement) => v.currentTime), { timeout: 20_000 })
    .toBeGreaterThan(0.5)
})

test('en modo red la API sin sesión responde 401', async ({ request }) => {
  const resp = await request.get(BASE + '/sources')
  expect(resp.status()).toBe(401)
})
```

- [ ] **Step 3: Correr el e2e completo**

Run: `cd web && npx playwright test --project=chromium; cd .. && git checkout -- internal/ui/dist/.gitkeep`
Expected: todas las specs PASS (las existentes y `modo-red.spec.ts`).

Si `npx playwright test` lista proyectos adicionales configurados (firefox), correr también `--project=firefox`.

- [ ] **Step 4: Commit**

```bash
git add web/tests/e2e/global-setup.ts web/tests/e2e/modo-red.spec.ts
git commit -m "test(e2e): modo red con clave, de la página de acceso a la reproducción

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 9: Imagen de Docker, publicación y humo en CI

**Files:**
- Create: `Dockerfile`, `goreleaser.Dockerfile`, `.dockerignore`, `compose.yaml`
- Modify: `.goreleaser.yml`, `.github/workflows/release.yml`, `.github/workflows/ci.yml`

**Interfaces:**
- Consumes: `open-tv healthcheck` (Tarea 6); modo red (Tarea 5).
- Produces: imagen `ghcr.io/gdberysan/open-tv` con tags `{{ .Version }}`, `{{ .Major }}.{{ .Minor }}`, `{{ .Major }}`, `latest` para `linux/amd64` y `linux/arm64`.

- [ ] **Step 1: `Dockerfile` (desde el código, para quien construye a mano y para el humo de CI)**

```dockerfile
# syntax=docker/dockerfile:1
# Imagen desde el código fuente. La publicada en ghcr.io la construye
# goreleaser con los binarios del release (goreleaser.Dockerfile); esta existe
# para construirla a mano y para la prueba de humo de CI.

FROM node:24-alpine AS cliente
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN mkdir -p ../internal/ui/dist && npm run build

FROM golang:1.25-alpine AS servidor
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ cmd/
COPY internal/ internal/
COPY --from=cliente /src/internal/ui/dist/ internal/ui/dist/
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /open-tv ./cmd/open-tv

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=servidor /open-tv /open-tv
# distroless no tiene shell para mkdir/chown: /data se crea copiando un
# directorio vacío con dueño nonroot. Un volumen con nombre hereda ese dueño.
COPY --chown=65532:65532 assets/docker/data-vacio /data
ENV LISTEN_ADDR=0.0.0.0:8080 \
    DB_PATH=/data/open-tv.db
VOLUME ["/data"]
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 CMD ["/open-tv", "healthcheck"]
ENTRYPOINT ["/open-tv", "serve", "--no-browser"]
```

`go.mod` fija `go 1.25.x`: la etapa usa `golang:1.25-alpine`. Si `go.mod` sube de versión, subir la etiqueta a la vez.

Crear el directorio vacío versionado `assets/docker/data-vacio/.gitkeep` (fichero vacío).

- [ ] **Step 2: `goreleaser.Dockerfile`**

```dockerfile
# syntax=docker/dockerfile:1
# La usa goreleaser (dockers_v2): el binario ya viene compilado por plataforma
# en $TARGETPLATFORM/open-tv, con el cliente web embebido.
FROM gcr.io/distroless/static-debian12:nonroot
ARG TARGETPLATFORM
COPY ${TARGETPLATFORM}/open-tv /open-tv
COPY --chown=65532:65532 assets/docker/data-vacio /data
ENV LISTEN_ADDR=0.0.0.0:8080 \
    DB_PATH=/data/open-tv.db
VOLUME ["/data"]
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 CMD ["/open-tv", "healthcheck"]
ENTRYPOINT ["/open-tv", "serve", "--no-browser"]
```

`assets/docker/data-vacio/` (Step 1) llega al contexto de goreleaser por `extra_files` (Step 4).

- [ ] **Step 3: `.dockerignore` y `compose.yaml`**

`.dockerignore`:

```
.git
mobile
docs
assets/readme
web/node_modules
web/tests
internal/ui/dist
dist
open-tv
**/*_test.go
```

(`internal/ui/dist` se ignora a propósito: la etapa `cliente` lo construye. Pero el paquete `ui` hace `go:embed all:dist` y necesita el directorio: la etapa `servidor` lo recibe con `COPY --from=cliente`.)

`compose.yaml`:

```yaml
# Open TV accesible desde cualquier dispositivo de tu red.
# La primera vez, la clave de acceso sale en el log:  docker compose logs open-tv
# O fija la tuya con OPEN_TV_ACCESS_KEY.
services:
  open-tv:
    image: ghcr.io/gdberysan/open-tv:latest
    container_name: open-tv
    restart: unless-stopped
    ports:
      - "8080:8080"
    volumes:
      - open-tv-data:/data
    # environment:
    #   OPEN_TV_ACCESS_KEY: "pon-aqui-una-clave-larga"

volumes:
  open-tv-data:
```

- [ ] **Step 4: goreleaser `dockers_v2`**

En `.goreleaser.yml`, tras el bloque `checksum:` añadir:

```yaml
# Imagen multi-arquitectura en ghcr.io con los mismos binarios del release.
# El paquete de ghcr.io nace PRIVADO: tras el primer release hay que hacerlo
# público a mano (GitHub → Packages → open-tv → Package settings).
dockers_v2:
  - id: open-tv
    ids:
      - open-tv
    dockerfile: goreleaser.Dockerfile
    images:
      - "ghcr.io/gdberysan/open-tv"
    tags:
      - "{{ .Version }}"
      - "{{ .Major }}.{{ .Minor }}"
      - "{{ .Major }}"
      - "latest"
    platforms:
      - linux/amd64
      - linux/arm64
    extra_files:
      - assets/docker/data-vacio
    labels:
      org.opencontainers.image.title: "Korven Open TV"
      org.opencontainers.image.description: "Free-to-air TV from your own M3U lists"
      org.opencontainers.image.source: "https://github.com/gdberysan/open-tv"
      org.opencontainers.image.version: "{{ .Version }}"
      org.opencontainers.image.licenses: "MIT"
```

`extra_files` es una lista de cadenas (verificado contra `goreleaser jsonschema`, v2.18). Si el snapshot del Step 5 falla porque goreleaser no copia directorios en `extra_files`, sustituir el directorio por un fichero `assets/docker/data-vacio/.gitkeep` en `extra_files` y en `goreleaser.Dockerfile` `COPY --chown=65532:65532 assets/docker/data-vacio/.gitkeep /data/.gitkeep` (crea `/data` con el mismo dueño).

Run: `goreleaser check`
Expected: `1 configuration file(s) validated`, sin avisos de deprecación.

Run: `HOMEBREW_TAP_KEY_PATH=/dev/null goreleaser release --snapshot --clean --skip=publish`
Expected: `release succeeded`, e imágenes `ghcr.io/gdberysan/open-tv:*-SNAPSHOT-*` construidas (requiere Docker en marcha).

- [ ] **Step 5: Probar la imagen de snapshot**

```bash
img=$(docker images --format '{{.Repository}}:{{.Tag}}' ghcr.io/gdberysan/open-tv | grep -- '-amd64\|arm64' | head -1)
docker run -d --name opentv-humo -p 18090:8080 -e OPEN_TV_ACCESS_KEY=humo "$img"
for i in $(seq 1 30); do curl -fsS 127.0.0.1:18090/health && break; sleep 1; done
curl -s -o /dev/null -w '%{http_code}\n' 127.0.0.1:18090/sources          # quiero 401
curl -s -o /dev/null -w '%{http_code}\n' -X POST -d clave=humo 127.0.0.1:18090/acceso   # quiero 303
docker inspect --format '{{.State.Health.Status}}' opentv-humo            # quiero healthy (tras ~30 s)
docker run --rm -d --name opentv-vol -v opentv-humo-vol:/data -e OPEN_TV_ACCESS_KEY=humo "$img"
sleep 5; docker logs opentv-vol 2>&1 | grep -c 'SQLite abierta'   # quiero 1: nonroot escribe en el volumen
docker rm -f opentv-humo opentv-vol; docker volume rm opentv-humo-vol
rm -rf dist
```

Si la imagen de snapshot se etiqueta distinto (sin sufijo de arquitectura), tomar la de la arquitectura local desde `docker images`.

- [ ] **Step 6: Workflow de release**

En `.github/workflows/release.yml`:

- En `permissions:` añadir `packages: write` junto a `contents: write`.
- Antes del paso `goreleaser/goreleaser-action`, añadir:

```yaml
      # dockers_v2 construye multi-arquitectura con buildx; QEMU emula arm64
      # en el runner amd64.
      - uses: docker/setup-qemu-action@v3
      - uses: docker/setup-buildx-action@v3
      - uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}
```

Comprobar que existen versiones más nuevas de esas tres acciones (`gh api repos/docker/setup-qemu-action/releases/latest -q .tag_name`, idem las otras) y usar la mayor disponible.

- [ ] **Step 7: Humo de Docker en CI**

En `.github/workflows/ci.yml`, añadir un job (junto a `web:`):

```yaml
  docker:
    name: Imagen de Docker
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v7
      - name: build
        run: docker build -t open-tv:ci .
      - name: humo
        run: |
          docker run -d --name humo -p 18090:8080 -e OPEN_TV_ACCESS_KEY=humo open-tv:ci
          for i in $(seq 1 30); do curl -fsS 127.0.0.1:18090/health && break; sleep 1; done
          test "$(curl -s -o /dev/null -w '%{http_code}' 127.0.0.1:18090/sources)" = 401
          test "$(curl -s -o /dev/null -w '%{http_code}' -X POST -d clave=humo 127.0.0.1:18090/acceso)" = 303
          test "$(curl -s -o /dev/null -w '%{http_code}' -X POST -d clave=mala 127.0.0.1:18090/acceso)" = 401
      - name: logs si falla
        if: failure()
        run: docker logs humo
```

- [ ] **Step 8: Construir la imagen desde el código en local**

Run:
```bash
docker build -t open-tv:local . && docker run -d --name opentv-local -p 18091:8080 open-tv:local
for i in $(seq 1 30); do curl -fsS 127.0.0.1:18091/health && break; sleep 1; done
docker logs opentv-local 2>&1 | grep -c '"clave"'     # quiero 1: la clave generada sale una vez
docker exec opentv-local /open-tv access-key          # imprime la misma clave
docker rm -f opentv-local
```
Expected: `/health` responde, la clave aparece una vez en el log y `access-key` la imprime.

- [ ] **Step 9: Commit**

```bash
git add Dockerfile goreleaser.Dockerfile .dockerignore compose.yaml assets/docker/data-vacio/.gitkeep .goreleaser.yml .github/workflows/release.yml .github/workflows/ci.yml
git commit -m "build(docker): imagen multi-arquitectura en ghcr.io y humo en CI

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 10: Documentación y cambio de invariante

**Files:**
- Modify: `README.md`, `README.es.md`, `SECURITY.md`, `CONTRIBUTING.md`, `CLAUDE.md`, `CHANGELOG.md`, `docs/superpowers/specs/2026-09-17-imagen-docker-design.md`
- Modify: `assets/readme/generar_banner.py` + regenerar `assets/readme/diagrama-{en,es}.svg` solo si el texto del diagrama deja de ser cierto (la caja «open-tv · listens on 127.0.0.1 only» pasa a «127.0.0.1 by default»).

- [ ] **Step 1: README.md — sección Docker y limitaciones**

Después de la sección `## Install` (antes de `## Features`), añadir:

````markdown
## Docker: watch from any device on your network

```bash
docker run -d --name open-tv -p 8080:8080 -v open-tv-data:/data \
  --restart unless-stopped ghcr.io/gdberysan/open-tv:latest
docker logs open-tv   # the access key is printed on first start
```

Open `http://<server-ip>:8080` on your TV, phone or laptop and paste the key.
Set your own with `-e OPEN_TV_ACCESS_KEY=…`, or show it again with
`docker exec open-tv /open-tv access-key`. A ready-to-use
[`compose.yaml`](compose.yaml) is in the repo. Images are built for
`linux/amd64` and `linux/arm64` (Raspberry Pi 4/5).

Outside `127.0.0.1`, Open TV always asks for the access key. To reach it from
outside your home, put it behind a reverse proxy with HTTPS (Caddy, Traefik,
nginx) rather than opening the port to the internet.
````

En `## Known limitations`, sustituir el punto «**No Docker image yet.** …» por nada (borrarlo), y en `## Privacy` sustituir «- The server binds to `127.0.0.1`, has no authentication, and isn't meant to be exposed to a network.» por:

```markdown
- By default the server binds to `127.0.0.1` and needs no login. Bind it
  anywhere else (as the Docker image does) and every request needs the
  access key.
```

En la tabla de variables, sustituir la fila de `LISTEN_ADDR` por:

```markdown
| `LISTEN_ADDR` | `127.0.0.1:8080` | Listen address. Anything other than loopback turns on network mode, which requires the access key. If the port is taken, the next free one is used. |
| `OPEN_TV_ACCESS_KEY` | generated | Access key for network mode. If unset, one is generated on first start, saved next to the database and printed once in the log. |
```

En el bloque de `## Commands and variables`, añadir:

```bash
open-tv access-key     # print the network-mode access key
open-tv healthcheck    # exit 0 if the local server is healthy (used by Docker)
```

En `## Features`, en el punto «**One binary, zero setup.** … No database to install, no Docker, no media center.» sustituir «no Docker» por «Docker optional».

En `## How it works`, sustituir la frase «and it never listens outside your machine.» por «and it only relays streams from your catalog.»

- [ ] **Step 2: README.es.md — el espejo**

Las mismas secciones, en español:

````markdown
## Docker: verlo desde cualquier dispositivo de tu red

```bash
docker run -d --name open-tv -p 8080:8080 -v open-tv-data:/data \
  --restart unless-stopped ghcr.io/gdberysan/open-tv:latest
docker logs open-tv   # la clave de acceso sale en el primer arranque
```

Abre `http://<ip-del-servidor>:8080` en la tele, el móvil o el portátil y pega
la clave. Fija la tuya con `-e OPEN_TV_ACCESS_KEY=…`, o vuelve a verla con
`docker exec open-tv /open-tv access-key`. En el repo hay un
[`compose.yaml`](compose.yaml) listo. Las imágenes son para `linux/amd64` y
`linux/arm64` (Raspberry Pi 4/5).

Fuera de `127.0.0.1`, Open TV siempre pide la clave de acceso. Para llegar a
él desde fuera de casa, ponlo detrás de un reverse proxy con HTTPS (Caddy,
Traefik, nginx) en vez de abrir el puerto a Internet.
````

Borrar el punto «**Sin imagen de Docker todavía.** …» de `## Limitaciones conocidas`. En `## Privacidad`:

```markdown
- Por defecto el servidor escucha en `127.0.0.1` y no pide acceso. Si escucha
  en cualquier otra dirección (como en la imagen de Docker), cada petición
  exige la clave de acceso.
```

Tabla de variables:

```markdown
| `LISTEN_ADDR` | `127.0.0.1:8080` | Dirección de escucha. Cualquier cosa que no sea loopback activa el modo red, que exige la clave de acceso. Si el puerto está ocupado, usa el siguiente libre. |
| `OPEN_TV_ACCESS_KEY` | generada | Clave de acceso del modo red. Si no se fija, se genera en el primer arranque, se guarda junto a la base de datos y sale una vez en el log. |
```

Comandos:

```bash
open-tv access-key     # muestra la clave de acceso del modo red
open-tv healthcheck    # sale con 0 si el servidor local está sano (lo usa Docker)
```

En `## Funciones`: «sin Docker» → «Docker opcional». En `## Cómo funciona`: «y nunca escucha fuera de tu máquina.» → «y solo retransmite streams de tu catálogo.»

- [ ] **Step 3: SECURITY.md**

En las dos secciones del modelo de amenaza (es y en), sustituir el punto del proxy por:

Español:
```markdown
- El **proxy HLS**, que solo relaya URLs del catálogo o firmadas por el propio
  proceso (HMAC), con guarda anti-SSRF (control de conexión, verificación en
  cada redirección y tope de tamaño).
- El **modo red**: fuera de loopback, todo salvo `/health` y `/acceso` exige
  una sesión firmada que se obtiene con la clave de acceso de la instalación.
```

Inglés:
```markdown
- The **HLS proxy**, which only relays catalog URLs or URLs signed by the
  process itself (HMAC), with SSRF guards (connection control, per-redirect
  checks, size cap).
- **Network mode**: outside loopback, everything except `/health` and
  `/acceso` requires a signed session obtained with the installation's access
  key.
```

En «Dentro del alcance» añadir: «- Saltarse la clave de acceso o la firma de sesión del modo red, o usar el proxy para relayar URLs que no están en el catálogo.»

- [ ] **Step 4: CONTRIBUTING.md**

Español: sustituir «el proxy local existe solo para los casos en que el navegador no puede con el flujo, y **escucha únicamente en loopback**.» por «el proxy local existe solo para los casos en que el navegador no puede con el flujo, y **solo relaya URLs del catálogo**.» y en «**Cuentas, registro o telemetría**» añadir al final «(el modo red usa una única clave de acceso por instalación, no cuentas)».

Inglés: «and it **listens on loopback only**.» → «and it **only relays catalog URLs**.», y tras «**Accounts, sign-up or telemetry**, not even "anonymous" kinds.» añadir « (network mode uses a single access key per installation, not accounts)».

- [ ] **Step 5: CLAUDE.md — invariante**

Sustituir el punto de Seguridad de «Invariantes DUROS»:

```markdown
- **Seguridad:** **sin autenticación solo en loopback**: cualquier listener
  no-loopback activa el modo red (`internal/acceso`), donde todo salvo
  `GET /health` y `/acceso` exige sesión firmada con la clave de la instalación.
  El **proxy HLS solo relaya URLs del catálogo o firmadas por el proceso**
  (`internal/proxy/firma.go`), con guarda SSRF (`controlConexion` +
  `checkRedirect` + tope de tamaño); middleware `MismoOrigen` (Host en
  loopback + `Sec-Fetch-Site`/`Origin` en métodos mutantes) contra
  CSRF/DNS-rebinding; CSP estricta (`script-src 'self'`; la página de acceso
  no lleva scripts). Reutiliza el cliente HTTP guardado del proxy para
  cualquier fetch server-side de streams.
```

En «Distribución», añadir:

```markdown
- **Imagen de Docker** en `ghcr.io/gdberysan/open-tv` por goreleaser
  `dockers_v2` (`goreleaser.Dockerfile`, binarios del release) y un
  `Dockerfile` desde el código para CI y builds a mano. El paquete de ghcr.io
  nace privado: tras el primer release con imagen hay que hacerlo público a
  mano.
```

- [ ] **Step 6: CHANGELOG.md**

Encima de `## [1.0.0] - 2026-09-16`:

```markdown
## [Sin publicar]

### Añadido

- **Modo red con clave de acceso**: fuera de `127.0.0.1`, Open TV exige una
  clave (generada en el primer arranque o fijada con `OPEN_TV_ACCESS_KEY`) y
  una sesión firmada. Página de acceso sin JavaScript, en español e inglés.
- **Imagen de Docker** multi-arquitectura (`linux/amd64`, `linux/arm64`) en
  `ghcr.io/gdberysan/open-tv`, con `compose.yaml` de ejemplo.
- Subcomandos `open-tv access-key` y `open-tv healthcheck`.

### Seguridad

- El proxy HLS deja de relayar URLs arbitrarias: solo las del catálogo o las
  que firma al reescribir un manifiesto.
- `MismoOrigen` comprueba también `Origin` en métodos mutantes.
```

Y abajo, antes de `[1.0.0]: …`: `[Sin publicar]: https://github.com/gdberysan/open-tv/compare/v1.0.0...HEAD`

- [ ] **Step 7: Spec y diagrama**

En la spec, cambiar `**Estado:** propuesta, NO aprobada.` por `**Estado:** aprobada por el dueño el 2026-09-17; implementada en la rama feat/modo-red-docker (plan docs/superpowers/plans/2026-09-17-modo-red-docker.md).`, y anotar las dos decisiones tomadas en el plan: sin `OPEN_TV_HOSTS` (en modo red se acepta cualquier Host; la sesión es el control de acceso) y sin `OPEN_TV_TRUSTED_PROXIES` (se confía en `X-Forwarded-Proto` solo para marcar `Secure`, que falsificado solo perjudica al que lo falsifica).

En `assets/readme/generar_banner.py`, en `DIAGRAMA`, cambiar `"app": ("open-tv", "listens on 127.0.0.1 only")` por `("open-tv", "127.0.0.1 · or network + key")`, `"app": ("open-tv", "escucha solo en 127.0.0.1")` por `("open-tv", "127.0.0.1 · o red + clave")`, y en las dos `label` sustituir «on 127.0.0.1» / «en 127.0.0.1» por «on 127.0.0.1 by default» / «en 127.0.0.1 por defecto». Regenerar: `python3 -m venv /tmp/banner && /tmp/banner/bin/pip install fonttools brotli && /tmp/banner/bin/python assets/readme/generar_banner.py`, y actualizar el `alt` de la imagen del diagrama en los dos README con el mismo cambio.

- [ ] **Step 8: Gates y commit**

Gates Go (scrubcheck incluido). Luego:

```bash
git add README.md README.es.md SECURITY.md CONTRIBUTING.md CLAUDE.md CHANGELOG.md docs/superpowers/specs/2026-09-17-imagen-docker-design.md assets/readme/generar_banner.py assets/readme/diagrama-en.svg assets/readme/diagrama-es.svg
git commit -m "docs: modo red, Docker y el nuevo invariante de seguridad

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

## Después de las tareas

1. Review final de la rama completa (modelo más capaz) con foco de seguridad: sesión, firma, limitador, `MismoOrigen`, página de acceso, imagen.
2. Gates completos + e2e + humo de Docker en verde.
3. Decisión del dueño: merge a `main`, tag `v1.1.0`, hacer público el paquete de ghcr.io, actualizar el issue #7 y el kit de lanzamiento.
