# ADR-001: ESTRATEGIA DE PARSING EPG

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
