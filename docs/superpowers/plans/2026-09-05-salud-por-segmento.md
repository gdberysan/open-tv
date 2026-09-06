# Salud por segmento — Plan de implementación

> **Para trabajadores agénticos:** SUB-SKILL OBLIGATORIA: usa
> `superpowers:subagent-driven-development` (recomendada) o
> `superpowers:executing-plans` para implementar tarea a tarea. Los pasos usan
> casillas (`- [ ]`).

**Goal:** Que el catálogo sepa qué códecs lleva de verdad cada mirror HLS,
que el reproductor no gaste 30 s en uno que ningún navegador decodifica, y
que el mensaje al usuario diga la verdad (caso medido: AMC (720p), vídeo
MPEG-2).

**Architecture:** Una segunda etapa de la pasada de salud lee la PAT/PMT de
los primeros 16 KB del primer segmento (solo si el mirror está vivo, es HLS y
su veredicto está caducado) con el cliente guardado del proxy, y guarda un
veredicto tri-estado `codec_ok` + `codecs` + `codec_checked_at` en
`streams`. `/channels/streams` lo expone (aditivo). El cliente salta los
mirrors `codecOk === false`, muestra el mensaje honesto al instante si no
queda ninguno, y diagnostica por `BUFFER_CODECS` cuando el servidor todavía
no sabe. Parser y clasificador son puros y viven en `internal/domain`.

**Tech Stack:** Go 1.2x (stdlib, SQLite modernc ya presente), Svelte 5 +
Vitest + Testing Library (ya presentes), hls.js 1.7.1 (evento
`BUFFER_CODECS`).

**Spec:** `docs/superpowers/specs/2026-09-05-salud-por-segmento-design.md`

## Global Constraints

- Código, comentarios, mensajes de commit y UI en **ESPAÑOL** (UI también en
  inglés, con paridad de claves: `web/src/i18n/i18n.test.ts` lo asevera).
- **Ninguna dependencia nueva**, ni Go ni npm.
- **`mobile/` CERO diffs.** Contrato de 15 claves de `/channels` intacto
  (`internal/domain/channel_test.go`). `/channels` y `/sources` NO cambian.
- **NUNCA `git add -A`.** Añade por ruta explícita.
- Identidad: `git config user.email` debe ser `gdberysan@gmail.com`. Trailer
  obligatorio en cada commit: `Co-Authored-By: <modelo en uso> <noreply@anthropic.com>`.
- Gates Go al final de CADA tarea Go: `gofmt -l .` (vacío) · `go vet ./...` ·
  `go build ./...` · `go test -race -count=1 ./...` · `golangci-lint run ./...`
  · `go run ./tools/scrubcheck` (exit 0).
- Gates Web al final de CADA tarea web: `cd web && npm run check && npm test
  && npm run build`. Bundle propio ≤ 80 KB gzip (lo imprime el build).
- Rama de trabajo: `feat/salud-por-segmento`, creada desde
  `spec/salud-por-segmento` (que ya contiene el spec).
- El cliente HTTP de los saltos derivados del manifiesto es SIEMPRE el
  guardado (`proxy.NuevoClienteGuardado`). El GET del manifiesto sigue con el
  cliente de siempre (fuera de alcance cambiarlo).
- Prefijo de sonda: **16 KB**. Caducidad del veredicto: **24 h**.
- Nunca `CodecNo` por no haber podido leer: cualquier error de lectura o
  parseo es `CodecUnknown`.

---

## Estructura de ficheros

**Crear:**
- `internal/domain/codec.go` — `CodecSupport`, `ClassifyCodecs`, `NombreCodecs`.
- `internal/domain/mpegts.go` — parser puro de PAT/PMT sobre un prefijo TS.
- `internal/domain/mpegts_test.go`, `internal/domain/codec_test.go`.
- `internal/proxy/cliente.go` — `NuevoClienteGuardado` + `guardiaRed`.
- `internal/proxy/cliente_test.go`.
- `internal/adapters/validator/sonda.go` — los saltos HTTP de la sonda.
- `internal/adapters/validator/sonda_test.go`.

**Modificar:**
- `internal/domain/stream.go` — `CodecCheckedAt`.
- `internal/ports/stream_repository.go` — campos en `StreamHealth` y `MirrorHealth`.
- `internal/adapters/db/schema.sql`, `internal/adapters/db/db.go` — columnas.
- `internal/adapters/db/stream_repository.go` (+ test) — columnas, `MarkBatch`, `FindMirrorsByChannelID`.
- `internal/proxy/handler.go`, `internal/proxy/handler_internal_test.go` — extracción del cliente.
- `internal/adapters/validator/models.go`, `checker.go`, `validator.go`, `worker.go` (+ tests).
- `internal/api/handlers/channel_handler.go`, `stats_handler.go` (+ tests).
- `web/src/datos/catalogo.ts`, `web/src/datos/http.ts` (+ test).
- `web/src/reproductor/diagnostico.ts` (+ test), `web/src/reproductor/failover.ts`.
- `web/src/i18n/es.ts`, `web/src/i18n/en.ts`.
- `web/src/componentes/Reproductor.svelte` (+ test).

---

### Task 0: Rama de trabajo

**Files:** ninguno.

- [ ] **Paso 1: Crear la rama desde la del spec**

```bash
cd "$(git rev-parse --show-toplevel)"
git checkout spec/salud-por-segmento
git checkout -b feat/salud-por-segmento
git config user.email   # debe imprimir gdberysan@gmail.com
```

---

### Task 1: Veredicto de códecs y parser MPEG-TS (dominio puro)

**Files:**
- Create: `internal/domain/codec.go`
- Create: `internal/domain/mpegts.go`
- Create: `internal/domain/codec_test.go`
- Create: `internal/domain/mpegts_test.go`
- Modify: `internal/domain/stream.go:16-31`

**Interfaces:**
- Produces:
  - `type CodecSupport int` con `CodecUnknown` (cero valor), `CodecNo`, `CodecOK`.
  - `type StreamTS struct { Tipo byte; PID uint16 }`
  - `func ParsearPMT(prefijo []byte) ([]StreamTS, error)`
  - `func ClassifyCodecs(streams []StreamTS) CodecSupport`
  - `func NombreCodecs(streams []StreamTS) string` — p. ej. `"mpeg2video,mp2"`.
  - `domain.Stream.CodecCheckedAt time.Time` (cero = nunca sondeado).

- [ ] **Paso 1: Escribir los tests del parser (fallan: no compila)**

`internal/domain/mpegts_test.go`:

```go
package domain_test

import (
	"testing"

	"github.com/gdberysan/open-tv/internal/domain"
)

// Constructores de paquetes TS sintéticos. No se copian bytes de orígenes de
// terceros: la PAT y la PMT se fabrican aquí con las longitudes correctas.

// paqueteTS envuelve una sección PSI en un paquete de 188 bytes con PUSI y
// pointer field 0. Relleno 0xff, como hace cualquier multiplexor.
func paqueteTS(pid int, seccion []byte) []byte {
	p := make([]byte, 188)
	for i := range p {
		p[i] = 0xff
	}
	p[0] = 0x47
	p[1] = 0x40 | byte(pid>>8) // payload_unit_start_indicator
	p[2] = byte(pid)
	p[3] = 0x10 // solo carga útil, continuity counter 0
	p[4] = 0    // pointer field
	copy(p[5:], seccion)
	return p
}

// paqueteTSConAdaptacion es igual pero con campo de adaptación de `n` bytes
// delante del pointer field (adaptation_field_control = 3).
func paqueteTSConAdaptacion(pid int, n int, seccion []byte) []byte {
	p := paqueteTS(pid, nil)
	p[3] = 0x30
	p[4] = byte(n)
	for i := 5; i < 5+n; i++ {
		p[i] = 0x00
	}
	p[5+n] = 0 // pointer field
	copy(p[6+n:], seccion)
	return p
}

// seccionPSI arma table_id + section_length + cuerpo + CRC ficticio.
func seccionPSI(tableID byte, cuerpo []byte) []byte {
	largo := len(cuerpo) + 4 // + CRC32
	s := []byte{tableID, 0xb0 | byte(largo>>8), byte(largo)}
	s = append(s, cuerpo...)
	return append(s, 0, 0, 0, 0)
}

func pat(pmtPID int) []byte {
	cuerpo := []byte{
		0x00, 0x01, // transport_stream_id
		0xc1,       // version 0, current_next 1
		0x00, 0x00, // section_number, last_section_number
		0x00, 0x01, // program_number 1
		0xe0 | byte(pmtPID>>8), byte(pmtPID),
	}
	return seccionPSI(0x00, cuerpo)
}

func pmt(streams ...domain.StreamTS) []byte {
	cuerpo := []byte{
		0x00, 0x01, // program_number 1
		0xc1,       // version 0, current_next 1
		0x00, 0x00, // section_number, last_section_number
		0xe1, 0x00, // PCR_PID 0x100
		0xf0, 0x00, // program_info_length 0
	}
	for _, s := range streams {
		cuerpo = append(cuerpo, s.Tipo, 0xe0|byte(s.PID>>8), byte(s.PID), 0xf0, 0x00)
	}
	return seccionPSI(0x02, cuerpo)
}

func segmento(paquetes ...[]byte) []byte {
	var out []byte
	for _, p := range paquetes {
		out = append(out, p...)
	}
	return out
}

func TestParsearPMT_MPEG2ConMP2(t *testing.T) {
	prefijo := segmento(
		paqueteTS(0, pat(0x1000)),
		paqueteTS(0x1000, pmt(domain.StreamTS{Tipo: 0x02, PID: 0x100}, domain.StreamTS{Tipo: 0x03, PID: 0x101})),
	)
	got, err := domain.ParsearPMT(prefijo)
	if err != nil {
		t.Fatalf("ParsearPMT: %v", err)
	}
	if len(got) != 2 || got[0].Tipo != 0x02 || got[0].PID != 0x100 || got[1].Tipo != 0x03 || got[1].PID != 0x101 {
		t.Errorf("streams = %+v", got)
	}
}

// La PAT y la PMT no tienen por qué ir en los dos primeros paquetes: puede
// haber paquetes de datos (sin PUSI) delante, y la PMT puede llevar campo de
// adaptación. Ambas cosas se ven en orígenes reales.
func TestParsearPMT_SaltaPaquetesDeDatosYCampoDeAdaptacion(t *testing.T) {
	datos := paqueteTS(0x100, nil)
	datos[1] = 0x01 // sin PUSI
	prefijo := segmento(
		datos,
		paqueteTS(0, pat(0x0fff)),
		datos,
		paqueteTSConAdaptacion(0x0fff, 7, pmt(domain.StreamTS{Tipo: 0x1b, PID: 0x0d3})),
	)
	got, err := domain.ParsearPMT(prefijo)
	if err != nil {
		t.Fatalf("ParsearPMT: %v", err)
	}
	if len(got) != 1 || got[0].Tipo != 0x1b {
		t.Errorf("streams = %+v, quiero solo H.264", got)
	}
}

func TestParsearPMT_PMTFueraDelPrefijoEsError(t *testing.T) {
	prefijo := segmento(paqueteTS(0, pat(0x1000)))
	if _, err := domain.ParsearPMT(prefijo); err == nil {
		t.Error("sin PMT en el prefijo tiene que fallar, no inventarse streams")
	}
}

func TestParsearPMT_SyncRotoEsError(t *testing.T) {
	prefijo := segmento(paqueteTS(0, pat(0x1000)))
	prefijo[0] = 0x00
	if _, err := domain.ParsearPMT(prefijo); err == nil {
		t.Error("un prefijo sin byte de sincronía no es MPEG-TS")
	}
}

func TestParsearPMT_PrefijoVacioEsError(t *testing.T) {
	if _, err := domain.ParsearPMT(nil); err == nil {
		t.Error("prefijo vacío tiene que fallar")
	}
}

// Descriptores en el bucle de ES (ES_info_length > 0): hay que saltarlos,
// no leerlos como si fueran otro stream.
func TestParsearPMT_SaltaDescriptoresDeES(t *testing.T) {
	cuerpo := []byte{
		0x00, 0x01, 0xc1, 0x00, 0x00, 0xe1, 0x00, 0xf0, 0x00,
		0x1b, 0xe1, 0x00, 0xf0, 0x03, 0x0a, 0x01, 0x00, // H.264 con 3 bytes de descriptor
		0x0f, 0xe1, 0x01, 0xf0, 0x00, // AAC
	}
	prefijo := segmento(paqueteTS(0, pat(0x1000)), paqueteTS(0x1000, seccionPSI(0x02, cuerpo)))
	got, err := domain.ParsearPMT(prefijo)
	if err != nil {
		t.Fatalf("ParsearPMT: %v", err)
	}
	if len(got) != 2 || got[0].Tipo != 0x1b || got[1].Tipo != 0x0f {
		t.Errorf("streams = %+v", got)
	}
}
```

`internal/domain/codec_test.go`:

```go
package domain_test

import (
	"testing"

	"github.com/gdberysan/open-tv/internal/domain"
)

func TestClassifyCodecs(t *testing.T) {
	casos := []struct {
		nombre  string
		streams []domain.StreamTS
		quiero  domain.CodecSupport
	}{
		{"MPEG-2 + MP2 (AMC mirror 1)", []domain.StreamTS{{Tipo: 0x02}, {Tipo: 0x03}}, domain.CodecNo},
		{"H.264 + AAC", []domain.StreamTS{{Tipo: 0x1b}, {Tipo: 0x0f}}, domain.CodecOK},
		{"H.264 solo, sin audio (AMC mirror 2)", []domain.StreamTS{{Tipo: 0x1b}}, domain.CodecOK},
		{"H.264 + AC-3: la regla es de vídeo", []domain.StreamTS{{Tipo: 0x1b}, {Tipo: 0x81}}, domain.CodecOK},
		{"HEVC + AAC", []domain.StreamTS{{Tipo: 0x24}, {Tipo: 0x0f}}, domain.CodecNo},
		{"MPEG-4 parte 2", []domain.StreamTS{{Tipo: 0x10}}, domain.CodecNo},
		{"VC-1", []domain.StreamTS{{Tipo: 0xea}}, domain.CodecNo},
		{"solo audio: no se juzga", []domain.StreamTS{{Tipo: 0x0f}}, domain.CodecUnknown},
		{"vacío", nil, domain.CodecUnknown},
	}
	for _, c := range casos {
		if got := domain.ClassifyCodecs(c.streams); got != c.quiero {
			t.Errorf("%s: ClassifyCodecs = %v, quiero %v", c.nombre, got, c.quiero)
		}
	}
}

func TestNombreCodecs(t *testing.T) {
	casos := []struct {
		streams []domain.StreamTS
		quiero  string
	}{
		{[]domain.StreamTS{{Tipo: 0x02}, {Tipo: 0x03}}, "mpeg2video,mp2"},
		{[]domain.StreamTS{{Tipo: 0x1b}, {Tipo: 0x0f}}, "h264,aac"},
		{[]domain.StreamTS{{Tipo: 0x1b}}, "h264"},
		{[]domain.StreamTS{{Tipo: 0x24}, {Tipo: 0x81}}, "hevc,ac3"},
		{[]domain.StreamTS{{Tipo: 0x1b}, {Tipo: 0x99}}, "h264,0x99"},
		{nil, ""},
	}
	for _, c := range casos {
		if got := domain.NombreCodecs(c.streams); got != c.quiero {
			t.Errorf("NombreCodecs(%+v) = %q, quiero %q", c.streams, got, c.quiero)
		}
	}
}
```

- [ ] **Paso 2: Ejecutar y ver el fallo**

Run: `go test ./internal/domain/ -run 'ParsearPMT|ClassifyCodecs|NombreCodecs' 2>&1 | head`
Expected: error de compilación (`undefined: domain.ParsearPMT`).

- [ ] **Paso 3: Implementar `codec.go`**

```go
package domain

import (
	"fmt"
	"strings"
)

// CodecSupport dice si el VÍDEO de un stream lo decodifica un navegador.
//
// Hermano de WebSupport y AirplaySupport en forma —tri-estado, "no se sabe"
// es un veredicto legítimo— pero con OTRO significado: WebNo dice «no
// directo, ve por el proxy»; CodecNo dice «ningún navegador lo decodifica,
// ni lo intentes». Caso medido (2026-09-05): AMC (720p) emite vídeo MPEG-2;
// hls.js lo tira en silencio y Safari da audio sin imagen.
type CodecSupport int

const (
	CodecUnknown CodecSupport = iota
	CodecNo
	CodecOK
)

// Tipos de stream elemental de MPEG-TS (ISO/IEC 13818-1, tabla 2-34, más
// los registrados por ATSC que ffmpeg y hls.js reconocen).
const (
	tsVideoMPEG2 byte = 0x02
	tsAudioMPEG1 byte = 0x03
	tsAudioMPEG2 byte = 0x04
	tsAudioAAC   byte = 0x0f
	tsVideoMPEG4 byte = 0x10
	tsAudioLATM  byte = 0x11
	tsVideoH264  byte = 0x1b
	tsVideoHEVC  byte = 0x24
	tsAudioAC3   byte = 0x81
	tsAudioEAC3  byte = 0x87
	tsVideoVC1   byte = 0xea
)

// ClassifyCodecs aplica una regla centrada en el vídeo: basta un stream
// H.264 para que sirva; si hay vídeo y ninguno es H.264, no sirve; sin
// vídeo no se juzga (el solo-audio no es asunto de este veredicto). HEVC
// queda fuera igual que en codecsWeb: solo Safari lo abre.
func ClassifyCodecs(streams []StreamTS) CodecSupport {
	hayVideo := false
	for _, s := range streams {
		switch s.Tipo {
		case tsVideoH264:
			return CodecOK
		case tsVideoMPEG2, tsVideoMPEG4, tsVideoHEVC, tsVideoVC1:
			hayVideo = true
		}
	}
	if hayVideo {
		return CodecNo
	}
	return CodecUnknown
}

// NombreCodecs devuelve una cadena corta para stats y para el mensaje al
// usuario, con los nombres que usa ffprobe. Un tipo desconocido sale en
// hexadecimal para que el censo lo pueda contar.
func NombreCodecs(streams []StreamTS) string {
	nombres := make([]string, 0, len(streams))
	for _, s := range streams {
		nombres = append(nombres, nombreTipoTS(s.Tipo))
	}
	return strings.Join(nombres, ",")
}

func nombreTipoTS(tipo byte) string {
	switch tipo {
	case tsVideoMPEG2:
		return "mpeg2video"
	case tsAudioMPEG1, tsAudioMPEG2:
		return "mp2"
	case tsAudioAAC, tsAudioLATM:
		return "aac"
	case tsVideoMPEG4:
		return "mpeg4"
	case tsVideoH264:
		return "h264"
	case tsVideoHEVC:
		return "hevc"
	case tsAudioAC3:
		return "ac3"
	case tsAudioEAC3:
		return "eac3"
	case tsVideoVC1:
		return "vc1"
	default:
		return fmt.Sprintf("0x%02x", tipo)
	}
}
```

- [ ] **Paso 4: Implementar `mpegts.go`**

```go
package domain

import (
	"errors"
	"fmt"
)

// StreamTS es una entrada del bucle de streams elementales de una PMT.
type StreamTS struct {
	Tipo byte
	PID  uint16
}

const (
	tamanoPaqueteTS = 188
	syncTS          = 0x47
	pidPAT          = 0
	tablaPAT        = 0x00
	tablaPMT        = 0x02
)

// ErrSinPMT: el prefijo era MPEG-TS válido pero la PAT o la PMT no cabían
// en él. Es un "no se sabe", no un "no sirve".
var ErrSinPMT = errors.New("mpegts: PAT/PMT no encontradas en el prefijo")

// ParsearPMT lee un PREFIJO de un segmento MPEG-TS (no hace falta el
// segmento entero: medido en orígenes reales, PAT y PMT van en los paquetes
// 2 y 3) y devuelve los streams elementales del primer programa. Cualquier
// cosa que no cuadre es error: quien llama lo traduce a CodecUnknown, nunca
// a CodecNo.
func ParsearPMT(prefijo []byte) ([]StreamTS, error) {
	n := len(prefijo) / tamanoPaqueteTS
	if n == 0 {
		return nil, ErrSinPMT
	}
	pmtPID := -1
	for i := 0; i < n; i++ {
		p := prefijo[i*tamanoPaqueteTS : (i+1)*tamanoPaqueteTS]
		if p[0] != syncTS {
			return nil, fmt.Errorf("mpegts: paquete %d sin byte de sincronía", i)
		}
		pusi := p[1]&0x40 != 0
		pid := int(p[1]&0x1f)<<8 | int(p[2])
		if !pusi {
			continue
		}
		seccion, ok := seccionDelPaquete(p)
		if !ok {
			continue
		}
		switch {
		case pid == pidPAT && pmtPID < 0:
			pmtPID = pidPMTDesdePAT(seccion)
		case pmtPID >= 0 && pid == pmtPID:
			return streamsDesdePMT(seccion)
		}
	}
	return nil, ErrSinPMT
}

// seccionDelPaquete salta la cabecera, el campo de adaptación si lo hay y el
// pointer field, y devuelve el principio de la sección PSI.
func seccionDelPaquete(p []byte) ([]byte, bool) {
	afc := (p[3] >> 4) & 0x3
	if afc == 0 || afc == 2 { // reservado / solo adaptación: sin carga útil
		return nil, false
	}
	off := 4
	if afc == 3 {
		off += 1 + int(p[4])
	}
	if off >= len(p) {
		return nil, false
	}
	off += 1 + int(p[off]) // pointer field
	if off >= len(p) {
		return nil, false
	}
	return p[off:], true
}

// pidPMTDesdePAT devuelve el PID de la PMT del primer programa distinto de 0
// (el 0 es la NIT). -1 si la sección no es una PAT o no tiene programas.
func pidPMTDesdePAT(sec []byte) int {
	if len(sec) < 12 || sec[0] != tablaPAT {
		return -1
	}
	largo := int(sec[1]&0x0f)<<8 | int(sec[2])
	fin := 3 + largo - 4 // sin el CRC32
	if fin > len(sec) {
		fin = len(sec)
	}
	for i := 8; i+4 <= fin; i += 4 {
		programa := int(sec[i])<<8 | int(sec[i+1])
		pid := int(sec[i+2]&0x1f)<<8 | int(sec[i+3])
		if programa != 0 {
			return pid
		}
	}
	return -1
}

func streamsDesdePMT(sec []byte) ([]StreamTS, error) {
	if len(sec) < 12 || sec[0] != tablaPMT {
		return nil, errors.New("mpegts: la sección en el PID de la PMT no es una PMT")
	}
	largo := int(sec[1]&0x0f)<<8 | int(sec[2])
	fin := 3 + largo - 4
	if fin > len(sec) {
		return nil, errors.New("mpegts: PMT truncada en el prefijo")
	}
	infoPrograma := int(sec[10]&0x0f)<<8 | int(sec[11])
	var out []StreamTS
	for i := 12 + infoPrograma; i+5 <= fin; {
		tipo := sec[i]
		pid := uint16(sec[i+1]&0x1f)<<8 | uint16(sec[i+2])
		infoES := int(sec[i+3]&0x0f)<<8 | int(sec[i+4])
		out = append(out, StreamTS{Tipo: tipo, PID: pid})
		i += 5 + infoES
	}
	if len(out) == 0 {
		return nil, errors.New("mpegts: PMT sin streams elementales")
	}
	return out, nil
}
```

- [ ] **Paso 5: Añadir `CodecCheckedAt` a `domain.Stream`**

En `internal/domain/stream.go`, tras `LastChecked`:

```go
	// CodecCheckedAt es cuándo se sondeó por última vez el códec del primer
	// segmento (spec salud-por-segmento). Cero = nunca. Lo lee el
	// health-worker para decidir si el veredicto está caducado.
	CodecCheckedAt time.Time `json:"CodecCheckedAt"`
```

- [ ] **Paso 6: Tests en verde y gates**

Run: `go test -race -count=1 ./internal/domain/`
Expected: PASS.

Run: `gofmt -l . && go vet ./... && go build ./... && golangci-lint run ./... && go run ./tools/scrubcheck`
Expected: sin salida de gofmt, todo exit 0.

- [ ] **Paso 7: Commit**

```bash
git add internal/domain/codec.go internal/domain/mpegts.go internal/domain/codec_test.go internal/domain/mpegts_test.go internal/domain/stream.go
git commit -m "feat(domain): veredicto de códecs y parser de PAT/PMT sobre un prefijo TS

Co-Authored-By: <modelo en uso> <noreply@anthropic.com>"
```

---

### Task 2: Persistencia del veredicto (ports + SQLite)

**Files:**
- Modify: `internal/ports/stream_repository.go:10-31`
- Modify: `internal/adapters/db/schema.sql:82-104`
- Modify: `internal/adapters/db/db.go:100-120` (`alterMigrations`)
- Modify: `internal/adapters/db/stream_repository.go:32`, `:163-195`, `:240-260`, `:266-326`
- Test: `internal/adapters/db/stream_repository_test.go`

**Interfaces:**
- Consumes: `domain.CodecSupport`, `domain.Stream.CodecCheckedAt` (Task 1).
- Produces:
  - `ports.StreamHealth{ ..., Codec domain.CodecSupport, Codecs string, CodecSondeado bool }`
  - `ports.MirrorHealth{ ..., Codec domain.CodecSupport, Codecs string }`
  - Columnas `streams.codec_ok`, `streams.codecs`, `streams.codec_checked_at`.

- [ ] **Paso 1: Tests del repositorio (fallan)**

Añadir a `internal/adapters/db/stream_repository_test.go` (usa `repoConCanal`, que ya existe en ese fichero y devuelve `*sql.DB` para leer columnas crudas):

```go
func TestMarkBatchPersisteCodec(t *testing.T) {
	ctx := context.Background()
	stRepo, sqlDB := repoConCanal(t)

	err := stRepo.MarkBatch(ctx, []ports.StreamHealth{
		{StreamID: "s1", IsAlive: true, LatencyMs: 10, Codec: domain.CodecNo, Codecs: "mpeg2video,mp2", CodecSondeado: true},
		{StreamID: "s2", IsAlive: true, LatencyMs: 10}, // sin sondear
	})
	if err != nil {
		t.Fatalf("MarkBatch: %v", err)
	}

	var codecOK sql.NullInt64
	var codecs string
	var checkedAt int64
	if err := sqlDB.QueryRow(`SELECT codec_ok, codecs, codec_checked_at FROM streams WHERE id = 's1'`).Scan(&codecOK, &codecs, &checkedAt); err != nil {
		t.Fatalf("leyendo s1: %v", err)
	}
	if !codecOK.Valid || codecOK.Int64 != 0 || codecs != "mpeg2video,mp2" || checkedAt == 0 {
		t.Errorf("s1: codec_ok=%v codecs=%q checked_at=%d", codecOK, codecs, checkedAt)
	}
	if err := sqlDB.QueryRow(`SELECT codec_ok, codecs, codec_checked_at FROM streams WHERE id = 's2'`).Scan(&codecOK, &codecs, &checkedAt); err != nil {
		t.Fatalf("leyendo s2: %v", err)
	}
	if codecOK.Valid || codecs != "" || checkedAt != 0 {
		t.Errorf("s2 no se sondeó y no debe cambiar: codec_ok=%v codecs=%q checked_at=%d", codecOK, codecs, checkedAt)
	}
}

// Una sonda que corrió pero no pudo decidir (fMP4, timeout, PMT fuera del
// prefijo) sella codec_checked_at —para no reintentar cada hora— pero NO
// pisa el veredicto anterior.
func TestMarkBatchCodecUnknownSellaPeroNoPisa(t *testing.T) {
	ctx := context.Background()
	stRepo, sqlDB := repoConCanal(t)

	if err := stRepo.MarkBatch(ctx, []ports.StreamHealth{
		{StreamID: "s1", IsAlive: true, Codec: domain.CodecNo, Codecs: "mpeg2video,mp2", CodecSondeado: true},
	}); err != nil {
		t.Fatalf("MarkBatch 1: %v", err)
	}
	// Forzar un sello viejo para poder ver que el segundo MarkBatch lo renueva.
	if _, err := sqlDB.Exec(`UPDATE streams SET codec_checked_at = 1 WHERE id = 's1'`); err != nil {
		t.Fatal(err)
	}
	if err := stRepo.MarkBatch(ctx, []ports.StreamHealth{
		{StreamID: "s1", IsAlive: true, Codec: domain.CodecUnknown, CodecSondeado: true},
	}); err != nil {
		t.Fatalf("MarkBatch 2: %v", err)
	}
	var codecOK sql.NullInt64
	var codecs string
	var despues int64
	if err := sqlDB.QueryRow(`SELECT codec_ok, codecs, codec_checked_at FROM streams WHERE id = 's1'`).Scan(&codecOK, &codecs, &despues); err != nil {
		t.Fatal(err)
	}
	if !codecOK.Valid || codecOK.Int64 != 0 || codecs != "mpeg2video,mp2" {
		t.Errorf("Unknown pisó el veredicto: codec_ok=%v codecs=%q", codecOK, codecs)
	}
	if despues <= 1 {
		t.Errorf("codec_checked_at debía renovarse: %d", despues)
	}
}

func TestFindMirrorsByChannelIDDevuelveCodec(t *testing.T) {
	ctx := context.Background()
	stRepo, _ := repoConCanal(t)
	if err := stRepo.MarkBatch(ctx, []ports.StreamHealth{
		{StreamID: "s1", IsAlive: true, LatencyMs: 10, Codec: domain.CodecNo, Codecs: "mpeg2video,mp2", CodecSondeado: true},
		{StreamID: "s2", IsAlive: true, LatencyMs: 20},
	}); err != nil {
		t.Fatalf("MarkBatch: %v", err)
	}
	mirrors, err := stRepo.FindMirrorsByChannelID(ctx, "ch-1")
	if err != nil {
		t.Fatalf("FindMirrorsByChannelID: %v", err)
	}
	if len(mirrors) != 2 {
		t.Fatalf("mirrors = %d", len(mirrors))
	}
	if mirrors[0].Codec != domain.CodecNo || mirrors[0].Codecs != "mpeg2video,mp2" {
		t.Errorf("s1: %+v", mirrors[0])
	}
	if mirrors[1].Codec != domain.CodecUnknown || mirrors[1].Codecs != "" {
		t.Errorf("s2 sin sondear debe ser Unknown: %+v", mirrors[1])
	}
}

func TestFindAllDevuelveCodecCheckedAt(t *testing.T) {
	ctx := context.Background()
	stRepo, _ := repoConCanal(t)
	if err := stRepo.MarkBatch(ctx, []ports.StreamHealth{
		{StreamID: "s1", IsAlive: true, Codec: domain.CodecOK, Codecs: "h264,aac", CodecSondeado: true},
	}); err != nil {
		t.Fatalf("MarkBatch: %v", err)
	}
	todos, err := stRepo.FindAll(ctx)
	if err != nil {
		t.Fatalf("FindAll: %v", err)
	}
	porID := map[string]domain.Stream{}
	for _, s := range todos {
		porID[s.ID] = s
	}
	if porID["s1"].CodecCheckedAt.IsZero() {
		t.Error("s1 sondeado: CodecCheckedAt no puede ser cero")
	}
	if !porID["s2"].CodecCheckedAt.IsZero() {
		t.Error("s2 nunca sondeado: CodecCheckedAt debe ser cero")
	}
}
```

Comprueba que el fichero de test ya importa `database/sql`, `domain` y `ports`; si no, añádelos.

- [ ] **Paso 2: Ejecutar y ver el fallo**

Run: `go test ./internal/adapters/db/ -run 'Codec' 2>&1 | head`
Expected: error de compilación (campos `Codec`/`CodecSondeado` inexistentes).

- [ ] **Paso 3: Ports**

En `internal/ports/stream_repository.go`:

```go
type StreamHealth struct {
	StreamID  string
	IsAlive   bool
	LatencyMs int64
	Web domain.WebSupport
	// Codec es el veredicto de la sonda del primer segmento (spec
	// salud-por-segmento). CodecSondeado dice si la sonda CORRIÓ en este
	// chequeo: si corrió, se sella codec_checked_at aunque el veredicto sea
	// CodecUnknown; un Unknown nunca pisa un veredicto anterior.
	Codec         domain.CodecSupport
	Codecs        string
	CodecSondeado bool
}

type MirrorHealth struct {
	URL       string
	IsAlive   bool
	LatencyMs int64
	WebOK     domain.WebSupport
	// Codec: CodecNo = ningún navegador decodifica su vídeo; el cliente lo
	// salta. Codecs es la cadena corta para el mensaje ("mpeg2video,mp2").
	Codec  domain.CodecSupport
	Codecs string
}
```

(Conserva los comentarios que ya tenían `Web` y `WebOK`.)

- [ ] **Paso 4: Esquema y migración**

En `internal/adapters/db/schema.sql`, dentro de `CREATE TABLE IF NOT EXISTS streams`, tras `web_ok`:

```sql
    -- Veredicto de la sonda del primer segmento (PAT/PMT): NULL = sin sondear,
    -- 0 = ningún navegador decodifica su vídeo, 1 = H.264. codecs es la
    -- cadena corta para stats y mensaje; codec_checked_at (epoch s, 0 = nunca)
    -- es la caducidad: solo se vuelve a sondear pasadas 24 h.
    codec_ok         INTEGER CHECK (codec_ok IN (0,1)),
    codecs           TEXT    NOT NULL DEFAULT '',
    codec_checked_at INTEGER NOT NULL DEFAULT 0,
```

En `internal/adapters/db/db.go`, al final de la lista `alters`:

```go
		// Sonda de códecs por segmento (spec salud-por-segmento, 2026-09-05).
		"ALTER TABLE streams ADD COLUMN codec_ok INTEGER",
		"ALTER TABLE streams ADD COLUMN codecs TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE streams ADD COLUMN codec_checked_at INTEGER NOT NULL DEFAULT 0",
```

- [ ] **Paso 5: Repositorio**

`streamColumns` y `scanStream`:

```go
const streamColumns = ` id, channel_id, url, protocol, referrer, user_agent, latency_ms, is_alive, last_checked, codec_checked_at `
```

```go
func scanStream(rows *sql.Rows) (domain.Stream, error) {
	var (
		s              domain.Stream
		latencyMs      sql.NullInt64
		isAlive        int
		lastChecked    sql.NullInt64
		codecCheckedAt int64
	)
	if err := rows.Scan(
		&s.ID, (*string)(&s.ChannelID), &s.URL, (*string)(&s.Protocol),
		&s.Referrer, &s.UserAgent,
		&latencyMs, &isAlive, &lastChecked, &codecCheckedAt,
	); err != nil {
		return domain.Stream{}, err
	}
	s.LatencyMs = latencyMs.Int64
	s.IsAlive = isAlive == 1
	if lastChecked.Valid {
		s.LastChecked = time.Unix(lastChecked.Int64, 0)
	}
	if codecCheckedAt != 0 {
		s.CodecCheckedAt = time.Unix(codecCheckedAt, 0)
	}
	return s, nil
}
```

`MarkBatch`, sentencia de vivos (la de muertos no cambia: un mirror muerto no se sondea):

```go
	stmtVivo, err := tx.PrepareContext(ctx,
		`UPDATE streams
		 SET is_alive = 1, fail_count = 0, latency_ms = ?, last_checked = ?, updated_at = ?,
		     web_ok = COALESCE(?, web_ok),
		     codec_ok = COALESCE(?, codec_ok),
		     codecs = COALESCE(?, codecs),
		     codec_checked_at = CASE WHEN ? = 1 THEN ? ELSE codec_checked_at END
		 WHERE id = ?`)
```

y en el bucle:

```go
		if res.IsAlive {
			_, err = stmtVivo.ExecContext(ctx, res.LatencyMs, now, now, argWebOK(res.Web),
				argCodecOK(res.Codec), argCodecs(res), boolAInt(res.CodecSondeado), now, res.StreamID)
		}
```

Helpers junto a `argWebOK`:

```go
// argCodecOK y argCodecs: nil para "no se sabe", que el COALESCE deja
// intacto. codecs solo se escribe con un veredicto real.
func argCodecOK(v domain.CodecSupport) any {
	switch v {
	case domain.CodecOK:
		return int64(1)
	case domain.CodecNo:
		return int64(0)
	default:
		return nil
	}
}

func argCodecs(res ports.StreamHealth) any {
	if res.Codec == domain.CodecUnknown {
		return nil
	}
	return res.Codecs
}

func boolAInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}
```

`FindMirrorsByChannelID`:

```go
	const q = `SELECT url, is_alive, COALESCE(latency_ms, 0), web_ok, codec_ok, codecs
	           FROM streams WHERE channel_id = ?
	           ORDER BY is_alive DESC,
	                    CASE WHEN latency_ms IS NULL THEN 1 ELSE 0 END,
	                    latency_ms ASC`
```

y en el scan añade `codecOK sql.NullInt64` y `&m.Codecs`:

```go
		if err := rows.Scan(&m.URL, &aliveIn, &m.LatencyMs, &webOK, &codecOK, &m.Codecs); err != nil {
```

tras el `switch` de `webOK`:

```go
		switch {
		case !codecOK.Valid:
			m.Codec = domain.CodecUnknown
		case codecOK.Int64 == 1:
			m.Codec = domain.CodecOK
		default:
			m.Codec = domain.CodecNo
		}
```

- [ ] **Paso 6: Tests en verde y gates**

Run: `go test -race -count=1 ./internal/adapters/db/ ./internal/ports/...`
Expected: PASS (incluidos los tests de `web_ok` que ya existían).

Run: `go build ./... && go vet ./... && gofmt -l . && golangci-lint run ./... && go run ./tools/scrubcheck`
Expected: exit 0. Si `go build` falla por otros implementadores de
`StreamRepository`, NO hace falta tocarlos: solo se añaden campos a structs,
no métodos a la interfaz.

- [ ] **Paso 7: Commit**

```bash
git add internal/ports/stream_repository.go internal/adapters/db/schema.sql internal/adapters/db/db.go internal/adapters/db/stream_repository.go internal/adapters/db/stream_repository_test.go
git commit -m "feat(db): columnas codec_ok/codecs/codec_checked_at y veredicto en MirrorHealth

Co-Authored-By: <modelo en uso> <noreply@anthropic.com>"
```

---

### Task 3: Extraer el cliente HTTP guardado del proxy

**Files:**
- Create: `internal/proxy/cliente.go`
- Create: `internal/proxy/cliente_test.go`
- Modify: `internal/proxy/handler.go:70-120`, `:210-245` (quitar `checkRedirect`/`controlConexion` del Handler)
- Modify: `internal/proxy/handler_internal_test.go:20`, `:68`, `:80`

**Interfaces:**
- Produces: `func NuevoClienteGuardado(permitirDestinosPrivados bool) *http.Client`
  — sin timeout de cliente (lo pone el contexto), sin keep-alive, 4
  conexiones por host, control de conexión sobre la IP resuelta y
  `CheckRedirect` con tope de 5 saltos y filtro de destinos privados.

- [ ] **Paso 1: Test externo del cliente (falla)**

`internal/proxy/cliente_test.go`:

```go
package proxy_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gdberysan/open-tv/internal/proxy"
)

// httptest vive en 127.0.0.1: con la guardia activa NO se llega ni a abrir
// la conexión; con permitirDestinosPrivados=true (solo tests) sí.
func TestNuevoClienteGuardadoBloqueaLoopbackSalvoEnTests(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/x.ts", nil)

	if resp, err := proxy.NuevoClienteGuardado(false).Do(req); err == nil {
		_ = resp.Body.Close()
		t.Fatal("con la guardia activa, loopback debe bloquearse antes de conectar")
	} else if !strings.Contains(err.Error(), "destino privado") {
		t.Errorf("el error debe nombrar la causa: %v", err)
	}

	resp, err := proxy.NuevoClienteGuardado(true).Do(req)
	if err != nil {
		t.Fatalf("con permitirDestinosPrivados=true debe pasar: %v", err)
	}
	_ = resp.Body.Close()
}

func TestNuevoClienteGuardadoSinKeepAliveYConTopePorHost(t *testing.T) {
	tr, ok := proxy.NuevoClienteGuardado(false).Transport.(*http.Transport)
	if !ok {
		t.Fatal("el transporte debe ser *http.Transport")
	}
	if !tr.DisableKeepAlives || tr.MaxConnsPerHost != 4 {
		t.Errorf("DisableKeepAlives=%v MaxConnsPerHost=%d; quiero true y 4", tr.DisableKeepAlives, tr.MaxConnsPerHost)
	}
}
```

- [ ] **Paso 2: Ejecutar y ver el fallo**

Run: `go test ./internal/proxy/ -run NuevoClienteGuardado 2>&1 | head`
Expected: `undefined: proxy.NuevoClienteGuardado`.

- [ ] **Paso 3: Crear `cliente.go` moviendo la guardia**

```go
package proxy

import (
	"fmt"
	"net"
	"net/http"
	"syscall"
)

// guardiaRed son las dos defensas de red del proxy, separadas del Handler
// para que cualquier fetch server-side de streams (la sonda de códecs del
// health-check, por ejemplo) las reutilice tal cual: el mismo cliente, no una
// copia que se desvíe con el tiempo.
type guardiaRed struct {
	// privadasOK solo es true en tests: httptest vive en 127.0.0.1, que en
	// producción es exactamente lo que hay que bloquear.
	privadasOK bool
}

// NuevoClienteGuardado construye el cliente HTTP con el que el proxy relaya
// y con el que se hace cualquier petición a una URL dictada por un tercero
// (un manifiesto). Sin timeout de cliente: lo pone el contexto de cada
// petición. Sin keep-alive y con 4 conexiones por host, como el checker.
func NuevoClienteGuardado(permitirDestinosPrivados bool) *http.Client {
	g := guardiaRed{privadasOK: permitirDestinosPrivados}
	dialer := &net.Dialer{
		Timeout: tiempoPeticion,
		// Control se ejecuta con la IP YA resuelta, justo antes de que el
		// kernel abra la conexión — no la que destinoPrivado comprobó antes
		// de que el propio Transport volviera a resolver el hostname por su
		// cuenta. Sin esto, un DNS que cambie de respuesta entre esa
		// comprobación y la conexión real (rebinding) esquiva el filtro por
		// hostname.
		Control: g.controlConexion,
	}
	return &http.Client{
		Transport: &http.Transport{
			MaxConnsPerHost:   4,
			DisableKeepAlives: true,
			DialContext:       dialer.DialContext,
		},
		// El http.Client por defecto sigue redirecciones (hasta 10) sin
		// preguntar. Un origen que pasó el filtro puede responder 302 hacia
		// 127.0.0.1 o hacia el enlace-local de metadatos de una nube.
		CheckRedirect: g.checkRedirect,
	}
}

// checkRedirect se ejecuta en cada salto de una redirección 3xx, ANTES de que
// el cliente la siga.
func (g guardiaRed) checkRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= maxRedirecciones {
		return fmt.Errorf("demasiadas redirecciones (%d)", len(via))
	}
	if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
		return fmt.Errorf("esquema no permitido en redirección: %s", req.URL.Scheme)
	}
	if !g.privadasOK && destinoPrivado(req.Context(), req.URL.Hostname()) {
		return fmt.Errorf("redirección a destino no permitido: %s", req.URL.Hostname())
	}
	return nil
}

// controlConexion se ejecuta justo antes de que el sistema operativo abra la
// conexión TCP, con la dirección YA resuelta. Es la única comprobación que
// mira la IP con la que el kernel conecta de verdad, así que es la que de
// verdad cierra el DNS-rebinding.
func (g guardiaRed) controlConexion(_, address string, _ syscall.RawConn) error {
	if g.privadasOK {
		return nil
	}
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("dirección de conexión inválida: %w", err)
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return fmt.Errorf("dirección de conexión no es una IP: %s", host)
	}
	if ipPrivada(ip) {
		return fmt.Errorf("conexión bloqueada a destino privado: %s", ip)
	}
	return nil
}
```

En `handler.go`: borra los métodos `checkRedirect` y `controlConexion` del
`Handler` y el bloque `dialer := ...` / `h.client = &http.Client{...}` de
`NewHandler`; en su lugar:

```go
	h.client = NuevoClienteGuardado(permitirDestinosPrivados)
```

Quita `syscall` de los imports de `handler.go` si ya nadie lo usa (y `net`
si tampoco; `destinoPrivado`/`ipPrivada` siguen ahí y usan `net`, así que
probablemente se queda).

En `handler_internal_test.go`, sustituye las tres construcciones
`h := &Handler{privadasOK: X}` por `h := guardiaRed{privadasOK: X}`; las
llamadas `h.checkRedirect(...)` y `h.controlConexion(...)` no cambian.

- [ ] **Paso 4: Tests en verde y gates**

Run: `go test -race -count=1 ./internal/proxy/ ./internal/api/...`
Expected: PASS (el relay y las pruebas de caja blanca siguen verdes).

Run: `go build ./... && go vet ./... && gofmt -l . && golangci-lint run ./... && go run ./tools/scrubcheck`
Expected: exit 0.

- [ ] **Paso 5: Commit**

```bash
git add internal/proxy/cliente.go internal/proxy/cliente_test.go internal/proxy/handler.go internal/proxy/handler_internal_test.go
git commit -m "refactor(proxy): extraer NuevoClienteGuardado para reutilizar la guardia SSRF

Co-Authored-By: <modelo en uso> <noreply@anthropic.com>"
```

---

### Task 4: La sonda de códecs en el checker

**Files:**
- Create: `internal/adapters/validator/sonda.go`
- Create: `internal/adapters/validator/sonda_test.go`
- Modify: `internal/adapters/validator/models.go:9-30`, `:34-40`, `:44-50`
- Modify: `internal/adapters/validator/checker.go:30-60`, `:78-175`
- Modify: `internal/adapters/validator/validator.go:15-27`, `:52`

**Interfaces:**
- Consumes: `domain.ParsearPMT`, `domain.ClassifyCodecs`, `domain.NombreCodecs` (Task 1); `proxy.NuevoClienteGuardado` (Task 3).
- Produces:
  - `StreamResult{ ..., Codec domain.CodecSupport, Codecs string, CodecSondeado bool }`
  - `TareaCheck{ ..., CodecCaducado bool }`
  - `Config{ ..., PermitirDestinosPrivados bool }`
  - `func NewCheckerConSonda(client, sonda HTTPChecker, timeout time.Duration) *Checker`
    (`NewChecker(client, timeout)` = `NewCheckerConSonda(client, nil, timeout)`; sonda nil → `proxy.NuevoClienteGuardado(false)`).
  - `func (c *Checker) CheckTarea(ctx context.Context, t TareaCheck) StreamResult`
    — lo que ahora llama `Validator.Start`. `CheckConCabeceras`/`Check` siguen existiendo y NUNCA sondean.

- [ ] **Paso 1: Tests de la sonda (fallan)**

`internal/adapters/validator/sonda_test.go`:

```go
package validator_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gdberysan/open-tv/internal/adapters/validator"
	"github.com/gdberysan/open-tv/internal/domain"
	"github.com/gdberysan/open-tv/internal/proxy"
)

// Paquetes TS sintéticos (misma construcción que en domain/mpegts_test.go;
// se repite aquí porque los helpers de test no se exportan entre paquetes).
func paqueteTS(pid int, seccion []byte) []byte {
	p := make([]byte, 188)
	for i := range p {
		p[i] = 0xff
	}
	p[0] = 0x47
	p[1] = 0x40 | byte(pid>>8)
	p[2] = byte(pid)
	p[3] = 0x10
	p[4] = 0
	copy(p[5:], seccion)
	return p
}

func seccionPSI(tableID byte, cuerpo []byte) []byte {
	largo := len(cuerpo) + 4
	s := []byte{tableID, 0xb0 | byte(largo>>8), byte(largo)}
	s = append(s, cuerpo...)
	return append(s, 0, 0, 0, 0)
}

func segmentoTS(tipos ...byte) []byte {
	pat := seccionPSI(0x00, []byte{0x00, 0x01, 0xc1, 0x00, 0x00, 0x00, 0x01, 0xf0, 0x00}) // PMT en PID 0x1000
	cuerpo := []byte{0x00, 0x01, 0xc1, 0x00, 0x00, 0xe1, 0x00, 0xf0, 0x00}
	for i, tipo := range tipos {
		cuerpo = append(cuerpo, tipo, 0xe1, byte(i), 0xf0, 0x00)
	}
	out := append([]byte{}, paqueteTS(0, pat)...)
	return append(out, paqueteTS(0x1000, seccionPSI(0x02, cuerpo))...)
}

type origen struct {
	srv       *httptest.Server
	hits      map[string]*atomic.Int32
	cabeceras map[string]http.Header
	segmento  []byte
	ignoraRange bool
	cuerpoGrande bool
	conMap    bool
}

func nuevoOrigen(t *testing.T) *origen {
	t.Helper()
	o := &origen{hits: map[string]*atomic.Int32{}, cabeceras: map[string]http.Header{}, segmento: segmentoTS(0x02, 0x03)}
	for _, ruta := range []string{"/master.m3u8", "/media.m3u8", "/seg.ts"} {
		o.hits[ruta] = &atomic.Int32{}
	}
	o.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h, ok := o.hits[r.URL.Path]; ok {
			h.Add(1)
		}
		o.cabeceras[r.URL.Path] = r.Header.Clone()
		switch r.URL.Path {
		case "/master.m3u8":
			w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
			_, _ = w.Write([]byte("#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=5000000,RESOLUTION=1920x1080\nmedia.m3u8\n"))
		case "/media.m3u8":
			w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
			cuerpo := "#EXTM3U\n#EXT-X-TARGETDURATION:4\n#EXT-X-MEDIA-SEQUENCE:1\n"
			if o.conMap {
				cuerpo += "#EXT-X-MAP:URI=\"init.mp4\"\n"
			}
			cuerpo += "#EXTINF:4.0,\nseg.ts\n#EXTINF:4.0,\nseg2.ts\n"
			_, _ = w.Write([]byte(cuerpo))
		case "/seg.ts":
			w.Header().Set("Content-Type", "video/MP2T")
			if o.cuerpoGrande {
				// 4 MB de relleno TS válido tras la PMT: el cliente debe cortar a 16 KB.
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(o.segmento)
				relleno := make([]byte, 188)
				relleno[0] = 0x47
				relleno[1] = 0x01
				relleno[2] = 0x00
				relleno[3] = 0x10
				for i := 0; i < 4<<20/188; i++ {
					if _, err := w.Write(relleno); err != nil {
						return
					}
				}
				return
			}
			if rango := r.Header.Get("Range"); rango != "" && !o.ignoraRange {
				w.Header().Set("Content-Range", "bytes 0-375/4623108")
				w.WriteHeader(http.StatusPartialContent)
			}
			_, _ = w.Write(o.segmento)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(o.srv.Close)
	return o
}

func checkerDeTest() *validator.Checker {
	return validator.NewCheckerConSonda(nil, proxy.NuevoClienteGuardado(true), 3*time.Second)
}

func TestSondaMasterMediaSegmentoClasificaMPEG2(t *testing.T) {
	o := nuevoOrigen(t)
	res := checkerDeTest().CheckTarea(context.Background(), validator.TareaCheck{URL: o.srv.URL + "/master.m3u8", CodecCaducado: true})
	if !res.IsAlive {
		t.Fatalf("vivo esperado: %v", res.Error)
	}
	if !res.CodecSondeado || res.Codec != domain.CodecNo || res.Codecs != "mpeg2video,mp2" {
		t.Errorf("sondeado=%v codec=%v codecs=%q", res.CodecSondeado, res.Codec, res.Codecs)
	}
	if o.hits["/media.m3u8"].Load() != 1 || o.hits["/seg.ts"].Load() != 1 {
		t.Errorf("hits media=%d seg=%d, quiero 1 y 1", o.hits["/media.m3u8"].Load(), o.hits["/seg.ts"].Load())
	}
	if rango := o.cabeceras["/seg.ts"].Get("Range"); rango != "bytes=0-16383" {
		t.Errorf("Range = %q", rango)
	}
}

func TestSondaMediaPlaylistDirectaH264(t *testing.T) {
	o := nuevoOrigen(t)
	o.segmento = segmentoTS(0x1b, 0x0f)
	res := checkerDeTest().CheckTarea(context.Background(), validator.TareaCheck{URL: o.srv.URL + "/media.m3u8", CodecCaducado: true})
	if res.Codec != domain.CodecOK || res.Codecs != "h264,aac" {
		t.Errorf("codec=%v codecs=%q", res.Codec, res.Codecs)
	}
	if o.hits["/master.m3u8"].Load() != 0 {
		t.Error("una media playlist no debe provocar ningún salto a un master")
	}
}

func TestSondaOrigenQueIgnoraRangeSeCortaA16KB(t *testing.T) {
	o := nuevoOrigen(t)
	o.cuerpoGrande = true
	inicio := time.Now()
	res := checkerDeTest().CheckTarea(context.Background(), validator.TareaCheck{URL: o.srv.URL + "/media.m3u8", CodecCaducado: true})
	if res.Codec != domain.CodecNo {
		t.Errorf("codec=%v; con la PMT en los primeros bytes debe clasificar aunque el origen ignore el Range", res.Codec)
	}
	if time.Since(inicio) > 2*time.Second {
		t.Errorf("tardó %v: no puede estar leyendo los 4 MB", time.Since(inicio))
	}
}

func TestSondaFMP4QuedaDesconocidoPeroSondeado(t *testing.T) {
	o := nuevoOrigen(t)
	o.conMap = true
	res := checkerDeTest().CheckTarea(context.Background(), validator.TareaCheck{URL: o.srv.URL + "/media.m3u8", CodecCaducado: true})
	if !res.CodecSondeado || res.Codec != domain.CodecUnknown {
		t.Errorf("sondeado=%v codec=%v", res.CodecSondeado, res.Codec)
	}
	if o.hits["/seg.ts"].Load() != 0 {
		t.Error("con EXT-X-MAP no se pide ningún segmento")
	}
}

// El segmento lo dicta el manifiesto: con la guardia activa (producción),
// un segmento en loopback no se descarga y el veredicto es "no se sabe".
func TestSondaSegmentoEnDestinoPrivadoQuedaDesconocido(t *testing.T) {
	o := nuevoOrigen(t)
	c := validator.NewCheckerConSonda(nil, proxy.NuevoClienteGuardado(false), 3*time.Second)
	res := c.CheckTarea(context.Background(), validator.TareaCheck{URL: o.srv.URL + "/media.m3u8", CodecCaducado: true})
	if !res.IsAlive {
		t.Fatal("el manifiesto va por el cliente de siempre y debe dar vivo")
	}
	if res.Codec != domain.CodecUnknown || !res.CodecSondeado {
		t.Errorf("codec=%v sondeado=%v", res.Codec, res.CodecSondeado)
	}
	if o.hits["/seg.ts"].Load() != 0 {
		t.Error("la guardia debe cortar ANTES de conectar")
	}
}

func TestSondaMandaCabecerasDelStreamEnLosDosSaltos(t *testing.T) {
	o := nuevoOrigen(t)
	checkerDeTest().CheckTarea(context.Background(), validator.TareaCheck{
		URL: o.srv.URL + "/master.m3u8", Referrer: "https://ref.example/", UserAgent: "UA-Propio/1.0", CodecCaducado: true,
	})
	for _, ruta := range []string{"/media.m3u8", "/seg.ts"} {
		h := o.cabeceras[ruta]
		if h.Get("Referer") != "https://ref.example/" || h.Get("User-Agent") != "UA-Propio/1.0" {
			t.Errorf("%s: Referer=%q UA=%q", ruta, h.Get("Referer"), h.Get("User-Agent"))
		}
	}
}

func TestSondaNoCorreSiElVeredictoNoEstaCaducado(t *testing.T) {
	o := nuevoOrigen(t)
	res := checkerDeTest().CheckTarea(context.Background(), validator.TareaCheck{URL: o.srv.URL + "/master.m3u8", CodecCaducado: false})
	if !res.IsAlive || res.CodecSondeado {
		t.Errorf("vivo=%v sondeado=%v", res.IsAlive, res.CodecSondeado)
	}
	if o.hits["/media.m3u8"].Load() != 0 || o.hits["/seg.ts"].Load() != 0 {
		t.Error("sin caducidad no hay peticiones extra")
	}
}

func TestCheckConCabecerasNuncaSondea(t *testing.T) {
	o := nuevoOrigen(t)
	res := checkerDeTest().Check(context.Background(), o.srv.URL+"/master.m3u8")
	if res.CodecSondeado || o.hits["/seg.ts"].Load() != 0 {
		t.Error("Check/CheckConCabeceras conservan el comportamiento de siempre")
	}
}
```

- [ ] **Paso 2: Ejecutar y ver el fallo**

Run: `go test ./internal/adapters/validator/ -run Sonda 2>&1 | head`
Expected: `undefined: validator.NewCheckerConSonda` / `CheckTarea`.

- [ ] **Paso 3: Modelos**

En `models.go`:

```go
type StreamResult struct {
	URL       string
	IsAlive   bool
	LatencyMs int64
	Protocol  string
	Error     error
	StatusCode int
	Airplay domain.AirplaySupport
	Web domain.WebSupport

	// Codec/Codecs: veredicto de la sonda del primer segmento (PAT/PMT).
	// CodecSondeado = la sonda corrió en este chequeo, decidiera o no.
	Codec         domain.CodecSupport
	Codecs        string
	CodecSondeado bool
}

type TareaCheck struct {
	URL       string
	Referrer  string
	UserAgent string
	// CodecCaducado: el worker lo pone a true cuando el veredicto de códecs
	// de la fila no existe, tiene más de 24 h, o el mirror venía de muerto.
	// Solo entonces el checker sondea el segmento.
	CodecCaducado bool
}

type Config struct {
	MaxWorkers int
	Timeout time.Duration
	// PermitirDestinosPrivados solo es true en tests: httptest vive en
	// 127.0.0.1, que la guardia de la sonda bloquea en producción.
	PermitirDestinosPrivados bool
}
```

(Conserva los comentarios existentes de cada campo.)

- [ ] **Paso 4: Checker — constructor y separación manifiesto/sonda**

En `checker.go`:

```go
type Checker struct {
	client    HTTPChecker
	transport *http.Transport
	timeout   time.Duration
	// sonda es el cliente GUARDADO (proxy.NuevoClienteGuardado) con el que se
	// piden las URLs que dicta un manifiesto de terceros: media playlist y
	// segmento. El manifiesto en sí sigue yendo por client, como siempre.
	sonda HTTPChecker
}

func NewChecker(client HTTPChecker, timeout time.Duration) *Checker {
	return NewCheckerConSonda(client, nil, timeout)
}

// NewCheckerConSonda deja inyectar el cliente de la sonda. nil = el guardado
// de producción (bloquea destinos privados).
func NewCheckerConSonda(client, sonda HTTPChecker, timeout time.Duration) *Checker {
	var tr *http.Transport
	if client == nil {
		tr = &http.Transport{
			MaxConnsPerHost:   maxConnsPerHost,
			DisableKeepAlives: true,
		}
		client = &http.Client{Timeout: timeout, Transport: tr}
	}
	if sonda == nil {
		sonda = proxy.NuevoClienteGuardado(false)
	}
	return &Checker{client: client, transport: tr, timeout: timeout, sonda: sonda}
}
```

(Conserva el comentario sobre `DisableKeepAlives`.) Añade el import
`"github.com/gdberysan/open-tv/internal/proxy"`.

Refactor de `CheckConCabeceras`: el cuerpo actual pasa a
`comprobar(reqCtx, url, referrer, ua) (StreamResult, cuerpo string, final string)`,
SIN crear el contexto con timeout dentro (lo recibe), y devolviendo además
el cuerpo del manifiesto y la URL final cuando hubo GET (cadenas vacías si
solo hubo HEAD). Los dos envoltorios:

```go
func (c *Checker) CheckConCabeceras(ctx context.Context, url, referrer, userAgentStream string) StreamResult {
	reqCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	res, _, _ := c.comprobar(reqCtx, url, referrer, userAgentStream)
	return res
}

// CheckTarea es lo que ejecuta el pool: el chequeo de siempre y, si la
// tarea lo pide y el manifiesto está vivo, la sonda de códecs — dentro del
// MISMO timeout, para que la pasada no se alargue.
func (c *Checker) CheckTarea(ctx context.Context, t TareaCheck) StreamResult {
	reqCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	res, cuerpo, final := c.comprobar(reqCtx, t.URL, t.Referrer, t.UserAgent)
	if t.CodecCaducado && res.IsAlive && res.Protocol == "HLS" && cuerpo != "" {
		res.CodecSondeado = true
		res.Codec, res.Codecs = c.sondearCodecs(reqCtx, final, cuerpo, t.Referrer, t.UserAgent)
	}
	return res
}
```

Dentro de `comprobar`, en la rama del GET, tras clasificar:
`return result, string(cuerpo), final` (y `LatencyMs` se mide antes de
devolver, como ahora). En la rama HEAD sin fallback: `return result, "", ""`.

- [ ] **Paso 5: `sonda.go`**

```go
package validator

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/gdberysan/open-tv/internal/domain"
)

// maxBytesSonda es lo que se lee del primer segmento: 87 paquetes TS. Medido
// en orígenes reales, PAT y PMT van en los paquetes 2 y 3 (564 bytes).
const maxBytesSonda = 16 << 10

// maxBytesPlaylistSonda acota la media playlist intermedia (una playlist en
// vivo ronda el KB).
const maxBytesPlaylistSonda = 64 << 10

// sondearCodecs sigue el manifiesto hasta el primer segmento y lee su PMT.
// Devuelve CodecUnknown ante CUALQUIER problema: no poder leer no es "no
// sirve". Nunca lanza más de dos peticiones.
func (c *Checker) sondearCodecs(ctx context.Context, urlManifiesto, cuerpo, referrer, ua string) (domain.CodecSupport, string) {
	base, err := url.Parse(urlManifiesto)
	if err != nil {
		return domain.CodecUnknown, ""
	}
	media := cuerpo
	if esMaster(cuerpo) {
		u := resolverURI(base, primeraURI(cuerpo))
		if u == nil {
			return domain.CodecUnknown, ""
		}
		media, err = c.leerTexto(ctx, u, referrer, ua)
		if err != nil {
			return domain.CodecUnknown, ""
		}
		base = u
	}
	if strings.Contains(media, "#EXT-X-MAP") {
		// fMP4: el códec va en el init segment (moov/stsd). Fuera de alcance.
		return domain.CodecUnknown, ""
	}
	seg := resolverURI(base, primeraURI(media))
	if seg == nil {
		return domain.CodecUnknown, ""
	}
	prefijo, err := c.leerPrefijo(ctx, seg, referrer, ua)
	if err != nil || len(prefijo) == 0 || prefijo[0] != 0x47 {
		return domain.CodecUnknown, ""
	}
	streams, err := domain.ParsearPMT(prefijo)
	if err != nil {
		return domain.CodecUnknown, ""
	}
	return domain.ClassifyCodecs(streams), domain.NombreCodecs(streams)
}

func esMaster(cuerpo string) bool {
	return strings.Contains(cuerpo, "#EXT-X-STREAM-INF")
}

// primeraURI devuelve la primera línea que no es etiqueta ni está vacía: en
// un master es la primera variante, en una media playlist el primer segmento.
func primeraURI(cuerpo string) string {
	for _, linea := range strings.Split(cuerpo, "\n") {
		linea = strings.TrimSpace(linea)
		if linea == "" || strings.HasPrefix(linea, "#") {
			continue
		}
		return linea
	}
	return ""
}

func resolverURI(base *url.URL, uri string) *url.URL {
	if uri == "" {
		return nil
	}
	u, err := base.Parse(uri)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return nil
	}
	return u
}

func (c *Checker) peticionSonda(ctx context.Context, u *url.URL, referrer, ua string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	if ua == "" {
		ua = userAgentPorDefecto
	}
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Origin", domain.OrigenWeb)
	if referrer != "" {
		req.Header.Set("Referer", referrer)
	}
	return req, nil
}

func (c *Checker) leerTexto(ctx context.Context, u *url.URL, referrer, ua string) (string, error) {
	req, err := c.peticionSonda(ctx, u, referrer, ua)
	if err != nil {
		return "", err
	}
	resp, err := c.sonda.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", errStatus(resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxBytesPlaylistSonda))
	return string(b), err
}

// leerPrefijo pide los primeros 16 KB por Range. Si el origen lo ignora y
// contesta 200 con el segmento entero, se lee igualmente solo el prefijo y
// se cierra la conexión (sin keep-alive, cerrar aborta la descarga).
func (c *Checker) leerPrefijo(ctx context.Context, u *url.URL, referrer, ua string) ([]byte, error) {
	req, err := c.peticionSonda(ctx, u, referrer, ua)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Range", "bytes=0-16383")
	resp, err := c.sonda.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return nil, errStatus(resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxBytesSonda))
}

type errStatus int

func (e errStatus) Error() string { return "sonda: HTTP " + http.StatusText(int(e)) }
```

En `validator.go`, `Start` pasa a llamar `v.checker.CheckTarea(ctx, t)`, y
`NewValidator` construye el checker con la guardia según la config:

```go
	if checker == nil {
		checker = NewCheckerConSonda(nil, proxy.NuevoClienteGuardado(cfg.PermitirDestinosPrivados), cfg.Timeout)
	}
```

- [ ] **Paso 6: Tests en verde y gates**

Run: `go test -race -count=1 ./internal/adapters/validator/`
Expected: PASS (los tests antiguos del checker no cambian: `Check` y
`CheckConCabeceras` conservan su comportamiento).

Run: `go build ./... && go vet ./... && gofmt -l . && golangci-lint run ./... && go run ./tools/scrubcheck`
Expected: exit 0. Si gosec marca G107/G704 en la petición de la sonda, la
URL viene de un manifiesto de terceros y va por el cliente guardado, así que
anota `//nolint:gosec // URL de manifiesto; va por el cliente guardado (proxy.NuevoClienteGuardado)`
en la línea del `Do` (NO `#nosec`: golangci-lint no lo honra, lección de P1).

- [ ] **Paso 7: Commit**

```bash
git add internal/adapters/validator/sonda.go internal/adapters/validator/sonda_test.go internal/adapters/validator/models.go internal/adapters/validator/checker.go internal/adapters/validator/validator.go
git commit -m "feat(validator): sonda de códecs del primer segmento como segunda etapa del chequeo

Co-Authored-By: <modelo en uso> <noreply@anthropic.com>"
```

---

### Task 5: El worker decide la caducidad y persiste el veredicto

**Files:**
- Modify: `internal/adapters/validator/worker.go:55-120`
- Test: `internal/adapters/validator/worker_test.go:18-117` (fake) y nuevos tests

**Interfaces:**
- Consumes: `TareaCheck.CodecCaducado`, `StreamResult.Codec/Codecs/CodecSondeado` (Task 4); `domain.Stream.CodecCheckedAt` (Task 1); `ports.StreamHealth.Codec/Codecs/CodecSondeado` (Task 2).
- Produces: `const CaducidadCodec = 24 * time.Hour`; `func codecCaducado(s domain.Stream, ahora time.Time) bool` (no exportada).

- [ ] **Paso 1: Ampliar el fake del test y escribir los tests (fallan)**

En `worker_test.go`, el `fakeStreamRepo` guarda además los `StreamHealth`
completos:

```go
type fakeStreamRepo struct {
	mu      sync.Mutex
	streams []domain.Stream
	alive   map[string]int64
	dead    map[string]bool
	salud   map[string]ports.StreamHealth // último resultado por stream
}
```

en `newFakeStreamRepo` inicializa `salud: make(map[string]ports.StreamHealth)`,
y en `MarkBatch`, dentro del bucle y antes de delegar:

```go
		f.mu.Lock()
		f.salud[r.StreamID] = r
		f.mu.Unlock()
```

más un accesor:

```go
func (f *fakeStreamRepo) saludDe(id string) ports.StreamHealth {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.salud[id]
}
```

Tests nuevos (el helper `segmentoTS` de `sonda_test.go` está en el paquete
`validator_test`; `worker_test.go` es del paquete interno `validator`, así
que este test sirve el segmento como bytes crudos construidos aquí):

```go
// Prefijo TS mínimo con PAT + PMT (MPEG-2 + MP2), para no depender de los
// helpers del paquete externo de tests.
func prefijoMPEG2() []byte {
	paquete := func(pid int, seccion []byte) []byte {
		p := make([]byte, 188)
		for i := range p {
			p[i] = 0xff
		}
		p[0], p[1], p[2], p[3], p[4] = 0x47, 0x40|byte(pid>>8), byte(pid), 0x10, 0
		copy(p[5:], seccion)
		return p
	}
	seccion := func(tableID byte, cuerpo []byte) []byte {
		largo := len(cuerpo) + 4
		s := append([]byte{tableID, 0xb0 | byte(largo>>8), byte(largo)}, cuerpo...)
		return append(s, 0, 0, 0, 0)
	}
	pat := seccion(0x00, []byte{0x00, 0x01, 0xc1, 0x00, 0x00, 0x00, 0x01, 0xf0, 0x00})
	pmt := seccion(0x02, []byte{0x00, 0x01, 0xc1, 0x00, 0x00, 0xe1, 0x00, 0xf0, 0x00,
		0x02, 0xe1, 0x00, 0xf0, 0x00, 0x03, 0xe1, 0x01, 0xf0, 0x00})
	return append(paquete(0, pat), paquete(0x1000, pmt)...)
}

func origenHLS(t *testing.T, hitsSeg *atomic.Int32) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/media.m3u8":
			_, _ = w.Write([]byte("#EXTM3U\n#EXTINF:4.0,\nseg.ts\n"))
		case "/seg.ts":
			hitsSeg.Add(1)
			w.Header().Set("Content-Type", "video/MP2T")
			_, _ = w.Write(prefijoMPEG2())
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestWorker_SondeaSoloLosCaducados(t *testing.T) {
	var hitsSeg atomic.Int32
	srv := origenHLS(t, &hitsSeg)
	ahora := time.Now()

	nunca := stream("st-nunca", "ch-1", srv.URL+"/media.m3u8?a")
	reciente := stream("st-reciente", "ch-2", srv.URL+"/media.m3u8?b")
	reciente.IsAlive = true
	reciente.CodecCheckedAt = ahora.Add(-time.Hour)
	viejo := stream("st-viejo", "ch-3", srv.URL+"/media.m3u8?c")
	viejo.IsAlive = true
	viejo.CodecCheckedAt = ahora.Add(-25 * time.Hour)
	resucitado := stream("st-resucitado", "ch-4", srv.URL+"/media.m3u8?d")
	resucitado.IsAlive = false
	resucitado.CodecCheckedAt = ahora.Add(-time.Hour)

	repo := newFakeStreamRepo([]domain.Stream{nunca, reciente, viejo, resucitado})
	cfg := Config{MaxWorkers: 4, Timeout: 2 * time.Second, PermitirDestinosPrivados: true}
	w := NewWorker(repo, cfg, time.Hour, nil)
	w.checkOnce(context.Background())

	if got := hitsSeg.Load(); got != 3 {
		t.Errorf("segmentos pedidos = %d, quiero 3 (nunca, viejo, resucitado)", got)
	}
	for _, id := range []string{"st-nunca", "st-viejo", "st-resucitado"} {
		s := repo.saludDe(id)
		if !s.CodecSondeado || s.Codec != domain.CodecNo || s.Codecs != "mpeg2video,mp2" {
			t.Errorf("%s: %+v", id, s)
		}
	}
	if s := repo.saludDe("st-reciente"); s.CodecSondeado {
		t.Errorf("st-reciente no debía sondearse: %+v", s)
	}
}

// Dos filas con la MISMA URL: si cualquiera está caducada, la URL se sondea
// (una vez) y el veredicto se aplica a las dos.
func TestWorker_DedupPorURLSondeaSiAlgunaFilaEstaCaducada(t *testing.T) {
	var hitsSeg atomic.Int32
	srv := origenHLS(t, &hitsSeg)
	a := stream("st-a", "ch-1", srv.URL+"/media.m3u8")
	a.IsAlive = true
	a.CodecCheckedAt = time.Now()
	b := stream("st-b", "ch-2", srv.URL+"/media.m3u8") // nunca sondeado

	repo := newFakeStreamRepo([]domain.Stream{a, b})
	w := NewWorker(repo, Config{MaxWorkers: 4, Timeout: 2 * time.Second, PermitirDestinosPrivados: true}, time.Hour, nil)
	w.checkOnce(context.Background())

	if hitsSeg.Load() != 1 {
		t.Errorf("segmentos pedidos = %d, quiero 1", hitsSeg.Load())
	}
	for _, id := range []string{"st-a", "st-b"} {
		if s := repo.saludDe(id); !s.CodecSondeado || s.Codec != domain.CodecNo {
			t.Errorf("%s: %+v", id, s)
		}
	}
}

func TestCodecCaducado(t *testing.T) {
	ahora := time.Now()
	casos := []struct {
		nombre string
		s      domain.Stream
		quiero bool
	}{
		{"nunca sondeado", domain.Stream{IsAlive: true}, true},
		{"reciente y vivo", domain.Stream{IsAlive: true, CodecCheckedAt: ahora.Add(-time.Hour)}, false},
		{"más de 24 h", domain.Stream{IsAlive: true, CodecCheckedAt: ahora.Add(-CaducidadCodec - time.Minute)}, true},
		{"venía de muerto", domain.Stream{IsAlive: false, CodecCheckedAt: ahora.Add(-time.Hour)}, true},
	}
	for _, c := range casos {
		if got := codecCaducado(c.s, ahora); got != c.quiero {
			t.Errorf("%s: codecCaducado = %v, quiero %v", c.nombre, got, c.quiero)
		}
	}
}
```

Añade a los imports de `worker_test.go` lo que falte (`net/http`,
`net/http/httptest`, `sync/atomic`).

- [ ] **Paso 2: Ejecutar y ver el fallo**

Run: `go test ./internal/adapters/validator/ -run 'Sondea|CodecCaducado' 2>&1 | head`
Expected: `undefined: CaducidadCodec` / `codecCaducado`.

- [ ] **Paso 3: Implementar en `worker.go`**

```go
// CaducidadCodec: cuánto vale un veredicto de códecs antes de volver a
// sondear. Los códecs de un origen casi nunca cambian; 24 h acota el daño de
// un veredicto viejo sin gastar dos peticiones por mirror cada hora.
const CaducidadCodec = 24 * time.Hour

// codecCaducado decide si el chequeo de esta pasada debe sondear el segmento:
// nunca sondeado, veredicto viejo, o el mirror venía de muerto (un origen que
// vuelve puede haber cambiado de todo).
func codecCaducado(s domain.Stream, ahora time.Time) bool {
	if !s.IsAlive || s.CodecCheckedAt.IsZero() {
		return true
	}
	return ahora.Sub(s.CodecCheckedAt) > CaducidadCodec
}
```

En `checkOnce`, al construir `cabecerasPorURL`, la caducidad es un OR entre
las filas que comparten URL:

```go
	ahora := time.Now()
	for _, s := range streams {
		byURL[s.URL] = append(byURL[s.URL], s.ID)
		actual, ya := cabecerasPorURL[s.URL]
		if !ya {
			cabecerasPorURL[s.URL] = TareaCheck{URL: s.URL, Referrer: s.Referrer, UserAgent: s.UserAgent,
				CodecCaducado: codecCaducado(s, ahora)}
			continue
		}
		if actual.Referrer == "" && actual.UserAgent == "" && (s.Referrer != "" || s.UserAgent != "") {
			actual.Referrer, actual.UserAgent = s.Referrer, s.UserAgent
		}
		actual.CodecCaducado = actual.CodecCaducado || codecCaducado(s, ahora)
		cabecerasPorURL[s.URL] = actual
	}
```

(Conserva el comentario largo sobre el dedup de cabeceras.) Y al volcar
resultados:

```go
			resultados = append(resultados, ports.StreamHealth{
				StreamID:      id,
				IsAlive:       res.IsAlive,
				LatencyMs:     res.LatencyMs,
				Web:           res.Web,
				Codec:         res.Codec,
				Codecs:        res.Codecs,
				CodecSondeado: res.CodecSondeado,
			})
```

Añade `"github.com/gdberysan/open-tv/internal/domain"` a los imports de
`worker.go`. Al log final añade `slog.Int("codecs_sondeados", sondeados)`
contando los `res.CodecSondeado` por URL.

- [ ] **Paso 4: Tests en verde y gates**

Run: `go test -race -count=1 ./internal/adapters/validator/`
Expected: PASS, incluidos los tests antiguos del worker: sus filas nunca
tienen `CodecCheckedAt`, así que `CodecCaducado` es true, pero sus orígenes
falsos responden 200 con cuerpo VACÍO y `CheckTarea` exige `cuerpo != ""`
para sondear — cero peticiones extra, `TestWorker_MarcaVivosYMuertos` sigue
contando 1.

Run: `go build ./... && go vet ./... && gofmt -l . && golangci-lint run ./... && go run ./tools/scrubcheck`
Expected: exit 0.

- [ ] **Paso 5: Commit**

```bash
git add internal/adapters/validator/worker.go internal/adapters/validator/worker_test.go
git commit -m "feat(validator): el worker sondea códecs solo con veredicto caducado y persiste el resultado

Co-Authored-By: <modelo en uso> <noreply@anthropic.com>"
```

---

### Task 6: API aditiva — `/channels/streams` y `/stats`

**Files:**
- Modify: `internal/api/handlers/channel_handler.go:177-225`
- Modify: `internal/api/handlers/stats_handler.go:48-70`
- Create: `internal/api/handlers/catalogo_stats_test.go`
- Test: `internal/api/handlers/channel_handler_test.go:481-505`, `internal/domain/channel_test.go` (solo se ejecuta)

**Interfaces:**
- Consumes: `ports.MirrorHealth.Codec/Codecs` (Task 2); columnas `codec_ok` (Task 2).
- Produces: en cada elemento de `GET /channels/streams`: `codec_ok` (`true|false|null`) y `codecs` (string). En `GET /stats` → `catalogo.codec_no` (int).

- [ ] **Paso 1: Tests (fallan)**

En `channel_handler_test.go`, amplía `TestGetChannelStreamsDevuelveMirrorsOrdenados`:

```go
	streams := &mockStreamRepo{mirrors: []ports.MirrorHealth{
		{URL: "https://a/x.m3u8", IsAlive: true, LatencyMs: 100, WebOK: domain.WebOK, Codec: domain.CodecNo, Codecs: "mpeg2video,mp2"},
		{URL: "https://b/x.m3u8", IsAlive: true, LatencyMs: 300, WebOK: domain.WebNo},
	}}
```

y al final:

```go
	if got[0]["codec_ok"] != false || got[0]["codecs"] != "mpeg2video,mp2" {
		t.Errorf("codec_ok/codecs mal mapeados en el primero: %v", got[0])
	}
	if got[1]["codec_ok"] != nil || got[1]["codecs"] != "" {
		t.Errorf("sin sondear debe ser null y '': %v", got[1])
	}
```

No existe ningún test de `DBCatalogoStats`. Crea
`internal/api/handlers/catalogo_stats_test.go` en el paquete INTERNO
`handlers` (como `channel_handler_test.go`, que también abre una SQLite real):

```go
package handlers

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/gdberysan/open-tv/internal/adapters/db"
	"github.com/gdberysan/open-tv/internal/domain"
	"github.com/gdberysan/open-tv/internal/ports"
)

// El agregado del catálogo cuenta los mirrors cuyo vídeo ningún navegador
// decodifica (codec_ok = 0), aparte de web_ok/web_no.
func TestDBCatalogoStatsCuentaCodecNo(t *testing.T) {
	sqlDB, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer func() { _ = sqlDB.Close() }()
	if _, err := sqlDB.Exec(`
		INSERT INTO providers (id, type, base_url, priority, is_active, created_at, updated_at)
		VALUES ('opensource', 'opensource', 'http://test.invalid/index.m3u', 100, 1, 0, 0)
	`); err != nil {
		t.Fatalf("seed provider: %v", err)
	}
	ctx := context.Background()
	chRepo := db.NewChannelRepository(sqlDB)
	if err := chRepo.Save(ctx, domain.Channel{ID: "c1", Name: "AMC (720p)", ProviderID: "opensource", ProviderType: domain.ProviderOpenSource}); err != nil {
		t.Fatalf("Save canal: %v", err)
	}
	stRepo := db.NewStreamRepository(sqlDB)
	for _, id := range []string{"s-mpeg2", "s-h264", "s-sin-sondear"} {
		if err := stRepo.Save(ctx, domain.Stream{ID: id, ChannelID: "c1", URL: "http://o/" + id + ".m3u8", Protocol: domain.ProtocolHLS}); err != nil {
			t.Fatalf("Save %s: %v", id, err)
		}
	}
	if err := stRepo.MarkBatch(ctx, []ports.StreamHealth{
		{StreamID: "s-mpeg2", IsAlive: true, Codec: domain.CodecNo, Codecs: "mpeg2video,mp2", CodecSondeado: true},
		{StreamID: "s-h264", IsAlive: true, Codec: domain.CodecOK, Codecs: "h264,aac", CodecSondeado: true},
		{StreamID: "s-sin-sondear", IsAlive: true},
	}); err != nil {
		t.Fatalf("MarkBatch: %v", err)
	}

	resumen, err := NewDBCatalogoStats(sqlDB).Resumen(ctx)
	if err != nil {
		t.Fatalf("Resumen: %v", err)
	}
	if resumen["codec_no"] != 1 {
		t.Errorf("codec_no = %v, quiero 1", resumen["codec_no"])
	}
	if resumen["streams_totales"] != 3 {
		t.Errorf("streams_totales = %v, quiero 3", resumen["streams_totales"])
	}
}
```

- [ ] **Paso 2: Ejecutar y ver el fallo**

Run: `go test ./internal/api/handlers/ -run 'GetChannelStreams|Catalogo' 2>&1 | head`
Expected: FAIL (claves ausentes / campos inexistentes).

- [ ] **Paso 3: Handler**

```go
type mirrorJSON struct {
	URL       string `json:"url"`
	IsAlive   bool   `json:"is_alive"`
	LatencyMs int64  `json:"latency_ms"`
	WebOK     *bool  `json:"web_ok"`   // null = sin comprobar
	CodecOK   *bool  `json:"codec_ok"` // null = sin sondear; false = ningún navegador decodifica su vídeo
	Codecs    string `json:"codecs"`   // "mpeg2video,mp2"; '' si no se sabe
}
```

en el bucle:

```go
		salida = append(salida, mirrorJSON{
			URL: m.URL, IsAlive: m.IsAlive, LatencyMs: m.LatencyMs, WebOK: webOKaPtr(m.WebOK),
			CodecOK: codecOKaPtr(m.Codec), Codecs: m.Codecs,
		})
```

y junto a `webOKaPtr`:

```go
func codecOKaPtr(v domain.CodecSupport) *bool {
	switch v {
	case domain.CodecOK:
		t := true
		return &t
	case domain.CodecNo:
		f := false
		return &f
	default:
		return nil
	}
}
```

En `stats_handler.go`, `Resumen`:

```go
	const q = `SELECT
		COUNT(*),
		COALESCE(SUM(is_alive), 0),
		COALESCE(SUM(CASE WHEN web_ok = 1 THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN web_ok = 0 THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN codec_ok = 0 THEN 1 ELSE 0 END), 0)
		FROM streams`
	var total, vivos, webOK, webNo, codecNo int
	if err := c.db.QueryRowContext(ctx, q).Scan(&total, &vivos, &webOK, &webNo, &codecNo); err != nil {
```

y en el mapa: `"codec_no": codecNo,`.

- [ ] **Paso 4: Tests en verde, contrato de 15 claves y gates**

Run: `go test -race -count=1 ./internal/api/... ./internal/domain/`
Expected: PASS. `internal/domain/channel_test.go` (15 claves) sigue verde:
no se ha tocado `/channels`.

Run: `go build ./... && go vet ./... && gofmt -l . && golangci-lint run ./... && go run ./tools/scrubcheck`
Expected: exit 0.

- [ ] **Paso 5: Commit**

```bash
git add internal/api/handlers/channel_handler.go internal/api/handlers/channel_handler_test.go internal/api/handlers/stats_handler.go internal/api/handlers/catalogo_stats_test.go
git commit -m "feat(api): codec_ok y codecs en /channels/streams; codec_no en /stats

Co-Authored-By: <modelo en uso> <noreply@anthropic.com>"
```

---

### Task 7: Cliente — el veredicto llega a `Mirror`

**Files:**
- Modify: `web/src/datos/catalogo.ts:49-55`
- Modify: `web/src/datos/http.ts:61-66`, `:172-183`
- Test: `web/src/datos/http.test.ts:125-138`

**Interfaces:**
- Produces: `Mirror.codecOk?: boolean | null` y `Mirror.codecs?: string`.
  Opcionales a propósito: quince literales de `Mirror` en tests no tienen
  por qué cambiar, y `undefined` significa lo mismo que `null` («el servidor
  no lo dijo»).

- [ ] **Paso 1: Test (falla)**

En `http.test.ts`, sustituye el test `mirrors traduce las claves del cable`:

```ts
  it('mirrors traduce las claves del cable, codec_ok incluido', async () => {
    vi.stubGlobal('fetch', vi.fn(async () =>
      respuesta([
        { url: 'https://a/x.m3u8', is_alive: true, latency_ms: 100, web_ok: true, codec_ok: false, codecs: 'mpeg2video,mp2' },
        { url: 'https://b/x.m3u8', is_alive: true, latency_ms: 300, web_ok: false, codec_ok: true, codecs: 'h264,aac' },
        { url: 'https://c/x.m3u8', is_alive: false, latency_ms: 0, web_ok: null, codec_ok: null, codecs: '' },
        { url: 'https://d/x.m3u8', is_alive: true, latency_ms: 50 }, // servidor viejo: sin claves
      ]),
    ))
    const mirrors = await crearHttpCatalog('').mirrors('c1')
    expect(mirrors).toHaveLength(4)
    expect(mirrors[0]).toEqual({ url: 'https://a/x.m3u8', vivo: true, latenciaMs: 100, webOk: true, codecOk: false, codecs: 'mpeg2video,mp2' })
    expect(mirrors[1].codecOk).toBe(true)
    expect(mirrors[2].vivo).toBe(false)
    expect(mirrors[2].webOk).toBeNull()
    expect(mirrors[2].codecOk).toBeNull()
    // "sin sondear" es un tercer estado: nunca false.
    expect(mirrors[3].codecOk).toBeNull()
    expect(mirrors[3].codecs).toBe('')
  })
```

- [ ] **Paso 2: Ejecutar y ver el fallo**

Run: `cd web && npx vitest run src/datos/http.test.ts`
Expected: FAIL (`codecOk` ausente en el objeto).

- [ ] **Paso 3: Implementar**

`catalogo.ts`:

```ts
/** Un mirror de un canal con su salud, para el failover. */
export interface Mirror {
  url: string
  vivo: boolean | null
  latenciaMs: number
  webOk: boolean | null
  /** Veredicto de la sonda del primer segmento: false = ningún navegador
   *  decodifica su vídeo (el failover lo salta). null/undefined = sin
   *  sondear. */
  codecOk?: boolean | null
  /** Cadena corta ("mpeg2video,mp2") para el mensaje; '' si no se sabe. */
  codecs?: string
}
```

`http.ts`:

```ts
interface MirrorCable {
  url: string
  is_alive?: boolean
  latency_ms?: number
  web_ok?: boolean | null
  codec_ok?: boolean | null
  codecs?: string
}
```

y en `mirrors()`:

```ts
        webOk: m.web_ok ?? null,
        codecOk: m.codec_ok ?? null,
        codecs: m.codecs ?? '',
```

- [ ] **Paso 4: Gates web**

Run: `cd web && npm run check && npm test && npm run build`
Expected: todo verde; el tamaño gzip del bundle propio sigue por debajo de 80 KB.

- [ ] **Paso 5: Commit**

```bash
git add web/src/datos/catalogo.ts web/src/datos/http.ts web/src/datos/http.test.ts
git commit -m "feat(web): Mirror lleva codecOk/codecs desde /channels/streams

Co-Authored-By: <modelo en uso> <noreply@anthropic.com>"
```

---

### Task 8: Cliente — clase `codec`, textos y `via: 'ninguna'`

**Files:**
- Modify: `web/src/reproductor/diagnostico.ts:10-24`, `:46-80`
- Modify: `web/src/reproductor/failover.ts:36-46`
- Modify: `web/src/i18n/es.ts:120-132`, `web/src/i18n/en.ts:70-79`
- Modify: `web/src/componentes/Reproductor.svelte:397-412` (`claveDeClase`)
- Test: `web/src/reproductor/diagnostico.test.ts`

**Interfaces:**
- Produces:
  - `ClaseFallo` gana `'codec'`; `InfoFallo.sinVideo?: boolean`.
  - Claves i18n `reproductor.error.codec` (con `{codecs}`) y `reproductor.error.codecGenerico`.
  - `DesenlaceReproduccion.via: 'directo' | 'proxy' | 'ninguna'`.

- [ ] **Paso 1: Tests (fallan)**

En `diagnostico.test.ts`, dentro de `describe('clasificarFallo')`:

```ts
  // hls.js solo vio pistas de audio (BUFFER_CODECS sin vídeo): el vídeo va
  // en un formato que el demuxer tira en silencio (MPEG-2, caso AMC). Tiene
  // prioridad sobre 'inestable'/'formato': el atasco es CONSECUENCIA.
  it('sinVideo → codec, aunque el intento muriera por timeout', () => {
    expect(clasificarFallo({ sinVideo: true })).toBe('codec')
  })

  it('sinVideo gana a un mediaError de buffer', () => {
    expect(clasificarFallo({ sinVideo: true, tipoHls: 'mediaError', detallesHls: 'bufferStalledError' })).toBe('codec')
  })

  // Un status HTTP es más específico que la ausencia de vídeo: si el
  // manifiesto dio 404, el mirror caducó, hubiera visto lo que hubiera visto.
  it('404 gana a sinVideo', () => {
    expect(clasificarFallo({ sinVideo: true, httpStatus: 404 })).toBe('caducado')
  })

  it('sin la señal, nada cambia', () => {
    expect(clasificarFallo({ sinVideo: false, tipoHls: 'mediaError', detallesHls: 'bufferStalledError' })).toBe('inestable')
  })
```

Y en `describe('claseConsensuada')`:

```ts
  it("todos 'codec' → codec", () => {
    expect(claseConsensuada(['codec', 'codec'])).toBe('codec')
  })
```

- [ ] **Paso 2: Ejecutar y ver el fallo**

Run: `cd web && npx vitest run src/reproductor/diagnostico.test.ts`
Expected: FAIL (tipo/valor `'codec'` inexistente).

- [ ] **Paso 3: Implementar `diagnostico.ts`**

```ts
export type ClaseFallo = 'caido' | 'geo' | 'formato' | 'inestable' | 'caducado' | 'codec' | 'desconocido'

export interface InfoFallo {
  tipoHls?: string
  detallesHls?: string
  httpStatus?: number
  mediaErrorCode?: number
  /** hls.js emitió BUFFER_CODECS con pista de audio y SIN pista de vídeo: el
   *  vídeo viene en un formato que el demuxer tira en silencio (MPEG-2, caso
   *  AMC 720p). Lo anota el reproductor al fallar el intento. */
  sinVideo?: boolean
}
```

y en `clasificarFallo`, justo después de la regla de `'caido'` y ANTES de
la de `'formato'`:

```ts
  // El vídeo no llegó a existir para hls.js: ni 'inestable' ni 'formato'
  // (que aconseja Safari — y Safari tampoco lo decodifica).
  if (info.sinVideo) return 'codec'
```

Actualiza el comentario de reglas numeradas del encabezado añadiendo el
punto «3b. sinVideo → 'codec'».

- [ ] **Paso 4: i18n y `claveDeClase`**

`es.ts`, tras `reproductor.error.caducado`:

```ts
  // Sonda de códecs (spec 2026-09-05): el vídeo viene en un formato que
  // NINGÚN navegador decodifica (MPEG-2, típicamente). Sin consejo de
  // Safari a propósito: Safari da audio sin imagen.
  'reproductor.error.codec': 'El vídeo de este canal viene en {codecs}, un formato que ningún navegador decodifica. Solo lo puede ver un reproductor de escritorio (VLC o la app instalada).',
  'reproductor.error.codecGenerico': 'El vídeo de este canal viene en un formato que ningún navegador decodifica. Solo lo puede ver un reproductor de escritorio (VLC o la app instalada).',
```

`en.ts`, en el mismo sitio:

```ts
  'reproductor.error.codec': "This channel's video comes as {codecs}, a format no browser can decode. Only a desktop player (VLC or the installed app) can show it.",
  'reproductor.error.codecGenerico': "This channel's video comes in a format no browser can decode. Only a desktop player (VLC or the installed app) can show it.",
```

`Reproductor.svelte`, en `claveDeClase`:

```ts
      case 'codec':
        return 'reproductor.error.codecGenerico'
```

`failover.ts`:

```ts
  /** 'ninguna': no se llegó a intentar nada (todos los mirrors con
   *  codecOk === false). El backend agrupa por cadena libre. */
  via: 'directo' | 'proxy' | 'ninguna'
```

- [ ] **Paso 5: Gates web**

Run: `cd web && npm run check && npm test && npm run build`
Expected: verde, paridad es/en incluida.

- [ ] **Paso 6: Commit**

```bash
git add web/src/reproductor/diagnostico.ts web/src/reproductor/diagnostico.test.ts web/src/reproductor/failover.ts web/src/i18n/es.ts web/src/i18n/en.ts web/src/componentes/Reproductor.svelte
git commit -m "feat(web): clase de fallo 'codec' con mensaje honesto, sin consejo de Safari

Co-Authored-By: <modelo en uso> <noreply@anthropic.com>"
```

---

### Task 9: Cliente — saltar mirrors indecodificables y diagnosticar por `BUFFER_CODECS`

**Files:**
- Modify: `web/src/componentes/Reproductor.svelte:52-60`, `:431-560`, `:575-600`, `:1050-1059`
- Test: `web/src/componentes/Reproductor.test.ts:58-100` (doble de hls.js) y tests nuevos

**Interfaces:**
- Consumes: `Mirror.codecOk/codecs` (Task 7), `InfoFallo.sinVideo`, claves i18n, `via: 'ninguna'` (Task 8).
- Produces: estado `errorSinReintento` (interno); desenlace `{ resultado: 'fallo', motivo: 'codec', via: 'ninguna', mirrorIndex: 0 }` cuando no queda ningún mirror.

- [ ] **Paso 1: Ampliar el doble de hls.js**

En `Reproductor.test.ts`, en `hlsState` añade a la forma de cada instancia:

```ts
    /** Emite BUFFER_CODECS con las pistas que "vio" el demuxer. */
    pistas: (o: { video: boolean; audio: boolean }) => void
```

en `FakeHls.Events` añade `BUFFER_CODECS: 'buffer_codecs',` y en `loadSource`:

```ts
        pistas: (o: { video: boolean; audio: boolean }) =>
          this.oyentes['buffer_codecs']?.forEach((cb) =>
            cb('buffer_codecs', {
              ...(o.video ? { video: { container: 'video/mp4', codec: 'avc1.64001f' } } : {}),
              ...(o.audio ? { audio: { container: 'audio/mpeg', codec: '' } } : {}),
            }),
          ),
```

- [ ] **Paso 2: Tests nuevos (fallan)**

Añádelos dentro de `describe('Reproductor — failover entre mirrors')`:

```ts
  it('salta los mirrors con codecOk === false y prueba solo los demás', async () => {
    hlsState.instancias.length = 0
    const mirrors: Mirror[] = [
      { url: 'https://mpeg2/x.m3u8', vivo: true, latenciaMs: 100, webOk: true, codecOk: false, codecs: 'mpeg2video,mp2' },
      { url: 'https://h264/x.m3u8', vivo: true, latenciaMs: 200, webOk: true, codecOk: null },
    ]
    const fuente = { mirrors: vi.fn(async () => mirrors), proxyDisponible: vi.fn(async () => false) }
    const intentadas: string[] = []
    render(Reproductor, { canal, fuente: fuente as any, alIntentar: (url: string) => intentadas.push(url) })

    await vi.waitFor(() => expect(hlsState.instancias).toHaveLength(1))
    expect(intentadas).toEqual(['https://h264/x.m3u8'])
  })

  it('con todos los mirrors indecodificables muestra el mensaje de códec al instante, sin intentar ni botón de reintento', async () => {
    hlsState.instancias.length = 0
    const mirrors: Mirror[] = [
      { url: 'https://mpeg2/x.m3u8', vivo: true, latenciaMs: 100, webOk: false, codecOk: false, codecs: 'mpeg2video,mp2' },
      { url: 'https://hevc/x.m3u8', vivo: true, latenciaMs: 500, webOk: false, codecOk: false, codecs: 'hevc,aac' },
    ]
    const fuente = { mirrors: vi.fn(async () => mirrors), proxyDisponible: vi.fn(async () => true) }
    const desenlaces: DesenlaceReproduccion[] = []
    const intentadas: string[] = []
    render(Reproductor, {
      canal,
      fuente: fuente as any,
      alIntentar: (url: string) => intentadas.push(url),
      alDesenlace: (d: DesenlaceReproduccion) => desenlaces.push(d),
    })

    const esperado = t('reproductor.error.codec', { codecs: 'mpeg2video,mp2' })
    await vi.waitFor(() => expect(screen.queryAllByText(esperado).length).toBeGreaterThan(0))
    expect(intentadas).toEqual([])
    expect(hlsState.instancias).toHaveLength(0)
    expect(screen.queryByText(t('reproductor.error.reintentar'))).toBeNull()
    expect(screen.queryByText(t('reproductor.error.mirrorsProbados', { n: 2 }))).toBeNull()
    expect(desenlaces).toEqual([
      { canalId: 'c1', resultado: 'fallo', motivo: 'codec', motor: 'hlsjs', via: 'ninguna', mirrorIndex: 0 },
    ])
  })

  it('si el servidor no sabía y hls.js solo vio audio, el fallo se clasifica como codec', async () => {
    hlsState.instancias.length = 0
    const mirrors: Mirror[] = [{ url: 'https://mpeg2/x.m3u8', vivo: true, latenciaMs: 100, webOk: true }]
    const fuente = { mirrors: vi.fn(async () => mirrors), proxyDisponible: vi.fn(async () => false) }
    const desenlaces: DesenlaceReproduccion[] = []
    render(Reproductor, { canal, fuente: fuente as any, alDesenlace: (d: DesenlaceReproduccion) => desenlaces.push(d) })

    await vi.waitFor(() => expect(hlsState.instancias).toHaveLength(1))
    hlsState.instancias[0].pistas({ video: false, audio: true })
    // El intento muere como muere de verdad: sin vídeo nunca avanza y el
    // guard lo declara fatal. Aquí se fuerza con un fatal cualquiera.
    hlsState.instancias[0].fallar('bufferStalledError')

    await vi.waitFor(() => expect(screen.queryAllByText(t('reproductor.error.codecGenerico')).length).toBeGreaterThan(0))
    expect(desenlaces.at(-1)?.motivo).toBe('codec')
  })

  it('con pista de vídeo vista, un fallo NO es codec', async () => {
    hlsState.instancias.length = 0
    const mirrors: Mirror[] = [{ url: 'https://h264/x.m3u8', vivo: true, latenciaMs: 100, webOk: true }]
    const fuente = { mirrors: vi.fn(async () => mirrors), proxyDisponible: vi.fn(async () => false) }
    const desenlaces: DesenlaceReproduccion[] = []
    render(Reproductor, { canal, fuente: fuente as any, alDesenlace: (d: DesenlaceReproduccion) => desenlaces.push(d) })

    await vi.waitFor(() => expect(hlsState.instancias).toHaveLength(1))
    hlsState.instancias[0].pistas({ video: true, audio: true })
    hlsState.instancias[0].fallar('bufferStalledError')

    await vi.waitFor(() => expect(desenlaces).toHaveLength(1))
    expect(desenlaces[0].motivo).toBe('inestable')
    expect(screen.queryByText(t('reproductor.error.codecGenerico'))).toBeNull()
  })
```

- [ ] **Paso 3: Ejecutar y ver el fallo**

Run: `cd web && npx vitest run src/componentes/Reproductor.test.ts -t 'codec'`
Expected: FAIL en los cuatro.

- [ ] **Paso 4: Implementar en `Reproductor.svelte`**

Estado, junto a `numMirrorsProbados`:

```ts
  // Error sin salida: reintentar no cambia los códecs de un origen, y un
  // botón que no puede arreglar nada es una promesa falsa (misma lección
  // que el contador de mirrors «con mejor salud»).
  let errorSinReintento = $state(false)
```

En `intentar()`, tras `infoUltimoError = {}`:

```ts
      // Lo que el demuxer de hls.js VIO: si al fallar solo había audio, el
      // vídeo iba en un formato que tira en silencio (MPEG-2). Se anota en
      // infoUltimoError al fallar, no antes: un canal de solo audio que sí
      // reproduce nunca pasa por aquí.
      const pistas = { video: false, audio: false }
```

En el `alFallar` del guard, en la rama no confirmada, ANTES de `reject`:

```ts
          zanjado = true
          if (pistas.audio && !pistas.video) infoUltimoError = { ...infoUltimoError, sinVideo: true }
          reject(new Error(mensaje))
```

En el bloque de hls.js, junto a los otros `hls.on`:

```ts
        hls.on(Hls.Events.BUFFER_CODECS, (_evt, data) => {
          if (data.video || data.audiovideo) pistas.video = true
          if (data.audio) pistas.audio = true
        })
```

En `reproducir()`: al principio, `errorSinReintento = false` junto a
`mensajeError = null`. Y dentro de `if (mirrors.length > 0) {`, antes de
`totalMirrors = mirrors.length`:

```ts
        // Los mirrors cuyo vídeo ningún navegador decodifica (sonda del
        // servidor) no se intentan: cada uno costaría el presupuesto entero
        // del guard para acabar en el mismo sitio.
        const reproducibles = mirrors.filter((m) => m.codecOk !== false)
        if (reproducibles.length === 0) {
          cargando = false
          errorSinReintento = true
          const codecs = mirrors.find((m) => m.codecs)?.codecs ?? ''
          mensajeError = codecs
            ? t('reproductor.error.codec', { codecs })
            : t('reproductor.error.codecGenerico')
          alDesenlace({ canalId: canal.id, resultado: 'fallo', motivo: 'codec', motor, via: 'ninguna', mirrorIndex: 0 })
          return
        }
        totalMirrors = reproducibles.length
        const proxyDisp = await fuente.proxyDisponible()
        if (destruido || miId !== intentoId) return
        intentos = planDeFailover(reproducibles, motor, proxyDisp)
```

Markup del error (línea ~1050):

```svelte
      <div class="estado error">
        <p class="mensaje">{mensajeError}</p>
        {#if numMirrorsProbados > 0}
          <p class="mirrors">{t('reproductor.error.mirrorsProbados', { n: numMirrorsProbados })}</p>
        {/if}
        {#if !errorSinReintento}
          <button type="button" class="probar-mirror" onclick={reintentar}>
            {t('reproductor.error.reintentar')}
          </button>
        {/if}
      </div>
```

Comprueba que `t()` interpola `{codecs}` igual que `{n}` (ver `i18n/index.ts`);
si la firma tipa los parámetros como `Record<string, string | number>`, pasa
`codecs` tal cual.

- [ ] **Paso 5: Tests en verde y gates**

Run: `cd web && npx vitest run src/componentes/Reproductor.test.ts`
Expected: PASS (los tests antiguos siguen verdes: sin `codecOk` nada se filtra).

Run: `cd web && npm run check && npm test && npm run build`
Expected: verde; bundle propio ≤ 80 KB gzip.

- [ ] **Paso 6: Commit**

```bash
git add web/src/componentes/Reproductor.svelte web/src/componentes/Reproductor.test.ts
git commit -m "feat(web): saltar mirrors indecodificables y diagnosticar por BUFFER_CODECS

Co-Authored-By: <modelo en uso> <noreply@anthropic.com>"
```

---

### Task 10: Integración real contra AMC (720p) y cierre

**Files:**
- Modify: `internal/ui/dist/` (salida del build; restaurar `.gitkeep` si el build lo borra)
- Modify: `docs/superpowers/plans/2026-09-05-salud-por-segmento.md` (esta sección, con la evidencia)

**Interfaces:** ninguna nueva. Es la comprobación de la spec §5 («no
automatizable aquí»), con evidencia, no con «debería».

- [ ] **Paso 1: Gates completos de las dos mitades**

```bash
cd "$(git rev-parse --show-toplevel)"
gofmt -l . ; go vet ./... && go build ./... && go test -race -count=1 ./... && golangci-lint run ./... && go run ./tools/scrubcheck
cd web && npm run check && npm test && npm run build && cd ..
cd mobile && flutter analyze && flutter test && cd .. && git status --short mobile/   # debe estar VACÍO
```

Expected: todo verde, `git status --short mobile/` sin salida.

- [ ] **Paso 2: Binario nuevo y relanzamiento del gateway**

```bash
cd "$(git rev-parse --show-toplevel)"
go build -o open-tv ./cmd/open-tv
git checkout -- internal/ui/dist/.gitkeep 2>/dev/null; ls internal/ui/dist/.gitkeep
pkill -f "open-tv serve" ; sleep 3
curl -s http://127.0.0.1:8080/ | grep -o 'index-[A-Za-z0-9_-]*\.js'
```

Expected: launchd relanza el binario nuevo y el hash del bundle es el que
acaba de construir `npm run build` (compáralo con `ls internal/ui/dist/assets/`).

- [ ] **Paso 3: Esperar la pasada de salud y leer la evidencia en SQLite**

La pasada arranca tras el primer sync. Espera al log
`Health-check completado` (o hasta 15 min) y consulta:

```bash
sqlite3 "$HOME/Library/Application Support/Korven Open TV/iptv.db" \
  "SELECT url, is_alive, codec_ok, codecs, codec_checked_at FROM streams WHERE channel_id = 'opensource-AMC (720p)';"
sqlite3 "$HOME/Library/Application Support/Korven Open TV/iptv.db" \
  "SELECT codec_ok, COUNT(*) FROM streams WHERE protocol='HLS' AND is_alive=1 GROUP BY codec_ok;"
```

Expected: el mirror `http://23.239.31.26:8989/amc/index.m3u8` con
`codec_ok = 0` y `codecs = 'mpeg2video,mp2'`; el mirror
`http://41.205.93.154/AMC/index.m3u8` con `codec_ok = 1` y `codecs = 'h264'`
(o `NULL` si su segmento tardó más que el timeout: es aceptable y se
documenta). El segundo SELECT da el reparto del catálogo: anótalo en el
ledger (es el primer censo real de códecs por PMT).

- [ ] **Paso 4: Comprobación en un Chrome REAL con la pestaña VISIBLE**

Abre `http://127.0.0.1:8080/?fresh=<hash>` en Chrome (ventana al frente:
`document.visibilityState === 'visible'`, ver la trampa en CLAUDE.md),
busca «AMC» en la lista lateral y sintoniza «AMC (720p)». Expected:

- Si los DOS mirrors tienen `codec_ok = 0`: el mensaje de códec sale al
  instante, sin «Conectando…», sin botón Reintentar, y `/stats` cuenta
  `por_motivo.codec` y `por_via.ninguna`.
- Si el segundo mirror tiene `codec_ok = 1` (H.264 sin audio, lento): el
  reproductor SOLO intenta ese, con «Se probaron 1 mirrors» al agotarse. Ese
  mirror es el item 3 de la hoja de ruta, no de este plan.

Guarda una captura en `docs/superpowers/evidencia/2026-09-05-amc-codec.png`
(crea el directorio) y pega la salida de los dos SELECT en el ledger de SDD
(`.superpowers/sdd/2026-09-05-salud-por-segmento/progress.md`).

- [ ] **Paso 5: Commit de cierre**

```bash
git add internal/ui/dist docs/superpowers/evidencia/2026-09-05-amc-codec.png
git commit -m "build(web): bundle con la sonda de códecs; evidencia real de AMC (720p)

Co-Authored-By: <modelo en uso> <noreply@anthropic.com>"
```

No mergear a `main` ni pushear: es decisión del dueño
(`superpowers:finishing-a-development-branch`).

---

## Autorrevisión del plan (hecha al escribirlo)

- **Cobertura de la spec:** §3.1 → Task 1; §3.2 → Tasks 3 y 4; §3.3 → Task 2
  y 5; §3.4 → Task 6; §3.5 (coste) → Task 5 (caducidad) y Task 4 (16 KB, un
  solo timeout); §4.1 → Task 7; §4.2 y §4.3 → Tasks 8 y 9; §5 → cada tarea y
  Task 10; §7 (fuera de alcance) → nada lo implementa, y `EXT-X-MAP` queda
  explícitamente en `CodecUnknown` (Task 4).
- **Tipos consistentes entre tareas:** `CodecSupport`/`CodecNo`/`CodecOK`/`CodecUnknown`;
  `StreamTS{Tipo, PID}`; `StreamHealth.Codec/Codecs/CodecSondeado`;
  `MirrorHealth.Codec/Codecs`; `TareaCheck.CodecCaducado`;
  `Config.PermitirDestinosPrivados`; `NewCheckerConSonda`; `CheckTarea`;
  `Mirror.codecOk/codecs`; `InfoFallo.sinVideo`; clase `'codec'`;
  `via: 'ninguna'`.
- **Sin placeholders:** cada paso de código lleva el código.
