# Riesgos y Mitigaciones

| # | Riesgo | Severidad | Mitigación en Go |
|---|---|---|---|
| 1 | **Race condition en validador** — múltiples goroutines escriben resultados simultáneamente | Alta | Canal `results chan StreamResult` con un único goroutine consumidor (fan-in). Sin mutex en escritura. |
| 2 | **OOM en parser de EPG** — XMLs >500MB cargados en memoria | Alta | `xml.Decoder` en modo streaming con `Token()`. Pool de buffers con `sync.Pool`. Procesar y descartar por entrada. |
| 3 | **Tokens de XtreamCodes expirados** — sesiones simultáneas con tokens obsoletos | Alta | `sync.RWMutex` sobre token + refresh proactivo antes del TTL. Patrón `singleflight` para evitar thundering herd en refresh. |
| 4 | **Rate limiting de fuentes públicas** — IPTV-org bloquea scraping agresivo | Media | Pool de goroutines acotado (`semaphore` con canal buffereado). Jitter aleatorio entre requests (100–500ms). Backoff exponencial con `context.WithTimeout`. |
| 5 | **URLs rotatorias** — tokens en URL que expiran cada N horas | Media | `GetStreamURL` nunca cachea la URL final, solo la resuelve bajo demanda. TTL corto (5min) en caché L1 para URLs. |
| 6 | **Goroutine leak en validador** — goroutines colgadas en streams lentos | Alta | `context.WithTimeout` en cada request de validación. `defer cancel()` inmediato tras crear contexto. |
| 7 | **Parseo de M3U gigantes** — listas con 50k+ entradas | Media | Parseo línea a línea con `bufio.Scanner`. `runtime.GOMAXPROCS` explícito. Pipeline: parsear → validar → persistir en etapas independientes. |
