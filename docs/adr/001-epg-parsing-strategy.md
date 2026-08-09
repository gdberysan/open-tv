# ADR-001: ESTRATEGIA DE PARSING EPG

> **ESTADO: REVERTIDO (2026-08-08).** La guía de programación se eliminó del
> producto. La decisión técnica de este ADR era correcta —el parser en streaming
> procesaba un XMLTV de 38 MB sin problema— pero el problema no era técnico sino
> de datos: la única fuente pública con ids compatibles con iptv-org cubre 478
> canales, de los que 465 son de India. Para un catálogo de 12 639 canales
> sesgado a US/IN/RU/DE/BR/ES, la parrilla salía vacía para casi cualquier canal
> que el usuario fuese a ver.
>
> Se conserva este documento como registro histórico. El código está en el
> historial de git; si algún día existe una fuente XMLTV con cobertura real,
> este ADR sigue siendo el punto de partida correcto.

**Contexto:** Los archivos XMLTV de guías de programación pueden superar 500MB. Algunas fuentes distribuyen EPG semanal con miles de canales y programas.

**Opciones evaluadas:**

```
OPCIÓN A — Carga total en memoria
┌─────────┐    ioutil.ReadAll    ┌──────────┐    Unmarshal    ┌──────────┐
│  HTTP   │ ─────────────────► │  []byte  │ ──────────────► │  []XMLTV │
│ Source  │                     │ ~500MB   │                  │ en RAM   │
└─────────┘                     └──────────┘                  └──────────┘
✗ OOM en instancias <1GB RAM
✗ Latencia hasta terminar descarga completa

OPCIÓN B — Streaming con xml.Decoder  ← DECISIÓN ADOPTADA
┌─────────┐   io.Reader    ┌───────────┐  Token()  ┌──────────┐  onEntry()  ┌──────┐
│  HTTP   │ ────────────► │ xml.Decoder│ ────────► │  Entry   │ ──────────► │  DB  │
│ Source  │                └───────────┘           │ (1 vez)  │             │ Save │
└─────────┘                                         └──────────┘             └──────┘
✓ RAM constante ~2MB independiente del tamaño del archivo
✓ Primera entrada disponible en DB en <1 segundo
✓ Cancelable en cualquier punto vía context
```

**Decisión:** Opción B. `xml.Decoder.Token()` en loop, con `sync.Pool` para reutilizar buffers de strings. El callback `onEntry` permite batch-insert cada N entradas (N=500) para reducir transacciones SQLite.

**Trade-off aceptado:** Mayor complejidad de código vs. Opción A. Mitigado con tests de integración usando fixtures de XMLs sintéticos de 1MB, 100MB y 500MB.
