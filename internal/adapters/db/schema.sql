-- Convenciones:
-- Timestamps: INTEGER Unix epoch
-- FKs:        Habilitadas con PRAGMA foreign_keys = ON al conectar

PRAGMA journal_mode = WAL;
PRAGMA synchronous   = NORMAL;
PRAGMA cache_size    = -32000;
PRAGMA foreign_keys  = ON;

-- ─────────────────────────────────────────
-- PROVEEDORES
-- ─────────────────────────────────────────
-- Cada fila es una fuente "bring your own" dada de alta por el usuario (ver
-- SourceRepository). type se queda fijo a 'opensource' para todo M3U — no
-- distingue fuentes entre sí, eso lo hace base_url. Ya no hay seed: una
-- instalación limpia arranca sin ninguna fila aquí.
CREATE TABLE IF NOT EXISTS providers (
    id           TEXT    PRIMARY KEY,
    type         TEXT    NOT NULL CHECK (type IN ('opensource')),
    base_url     TEXT    NOT NULL,
    credentials  TEXT,
    -- Nombre que el usuario le puso a la fuente al darla de alta.
    label        TEXT    NOT NULL DEFAULT '',
    -- Cómo se obtiene el M3U: 'url' (remoto) o 'file' (subido al datadir).
    -- No es el formato del catálogo, eso ya lo fija type.
    kind         TEXT    NOT NULL DEFAULT 'url',
    -- url-tvg declarada en la cabecera M3U de la fuente; '' = sin guía.
    tvg_url      TEXT    NOT NULL DEFAULT '',
    priority     INTEGER NOT NULL DEFAULT 100,
    is_active    INTEGER NOT NULL DEFAULT 1 CHECK (is_active IN (0,1)),
    created_at   INTEGER NOT NULL,
    updated_at   INTEGER NOT NULL
);

-- ─────────────────────────────────────────
-- CATEGORÍAS
-- ─────────────────────────────────────────
CREATE TABLE IF NOT EXISTS categories (
    id            TEXT    PRIMARY KEY,
    name          TEXT    NOT NULL,
    provider_type TEXT    NOT NULL CHECK (provider_type IN ('opensource')),
    created_at    INTEGER NOT NULL,
    updated_at    INTEGER NOT NULL
);

-- ─────────────────────────────────────────
-- CANALES
-- ─────────────────────────────────────────
CREATE TABLE IF NOT EXISTS channels (
    id            TEXT    PRIMARY KEY,
    tvg_id        TEXT,   -- identificador XMLTV que une el canal con su EPG
    name          TEXT    NOT NULL,
    logo_url      TEXT,
    category_id   TEXT    REFERENCES categories(id) ON DELETE SET NULL,
    language_code TEXT,
    country_code  TEXT,
    provider_id   TEXT    NOT NULL REFERENCES providers(id) ON DELETE CASCADE,
    provider_type TEXT    NOT NULL CHECK (provider_type IN ('opensource')),
    is_adult      INTEGER NOT NULL DEFAULT 0 CHECK (is_adult IN (0,1)),
    created_at    INTEGER NOT NULL,
    updated_at    INTEGER NOT NULL,
    -- Sellado en cada upsert del sync. Lo que quede por debajo del corte de un
    -- sync exitoso es que el proveedor ya no lo lista: se poda.
    last_seen_at  INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_channels_last_seen ON channels(provider_id, last_seen_at);
CREATE INDEX IF NOT EXISTS idx_channels_category ON channels(category_id);
CREATE INDEX IF NOT EXISTS idx_channels_country  ON channels(country_code);
CREATE INDEX IF NOT EXISTS idx_channels_provider ON channels(provider_id);
CREATE INDEX IF NOT EXISTS idx_channels_search   ON channels(name COLLATE NOCASE);
CREATE INDEX IF NOT EXISTS idx_channels_tvg      ON channels(tvg_id) WHERE tvg_id IS NOT NULL;

-- ─────────────────────────────────────────
-- STREAMS (1:N con channels)
-- ─────────────────────────────────────────
CREATE TABLE IF NOT EXISTS streams (
    id           TEXT    PRIMARY KEY,
    channel_id   TEXT    NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    url          TEXT    NOT NULL,
    protocol     TEXT    NOT NULL CHECK (protocol IN ('HLS','DASH','RTMP')),
    latency_ms   INTEGER,
    is_alive     INTEGER NOT NULL DEFAULT 0 CHECK (is_alive IN (0,1)),
    -- Fallos consecutivos del health-check. Solo al llegar a DeadFailThreshold
    -- se apaga is_alive: un blip de red no puede ocultar el canal una hora.
    fail_count   INTEGER NOT NULL DEFAULT 0,
    -- Reproducibilidad directa en navegador (veredicto estricto de hls.js).
    -- NULL = sin comprobar, y hay que distinguirlo de 0: la tarjeta pinta
    -- cosas distintas para "no se ve" y "todavía no lo sé".
    web_ok       INTEGER CHECK (web_ok IN (0,1)),
    last_checked INTEGER,
    created_at   INTEGER NOT NULL,
    updated_at   INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_streams_channel_alive ON streams(channel_id, is_alive);
CREATE INDEX IF NOT EXISTS idx_streams_latency       ON streams(channel_id, latency_ms) WHERE is_alive = 1;

-- ─────────────────────────────────────────
-- EPG (guía de programación)
-- ─────────────────────────────────────────
-- Vuelve en P2, esta vez ANCLADA a la fuente: el url-tvg lo declara cada
-- provider en su propia cabecera M3U (providers.tvg_url), y channel_id aquí
-- es el tvg-id del XMLTV de ESA fuente — el mismo valor que channels.tvg_id.
-- Dos providers pueden traer el mismo channel_id con guías distintas: por
-- eso la PK y el join de lectura SIEMPRE llevan provider_id, nunca channel_id
-- a secas. Ver EPGRepository.
CREATE TABLE IF NOT EXISTS epg_programmes (
    provider_id  TEXT    NOT NULL REFERENCES providers(id) ON DELETE CASCADE,
    channel_id   TEXT    NOT NULL,
    start_utc    INTEGER NOT NULL,
    stop_utc     INTEGER NOT NULL,
    title        TEXT    NOT NULL,
    sub_title    TEXT,
    description  TEXT,
    PRIMARY KEY (provider_id, channel_id, start_utc)
);
CREATE INDEX IF NOT EXISTS idx_epg_lookup ON epg_programmes (provider_id, channel_id, start_utc);

-- Nota: sync_log, user_agents y epg_entries existieron en esquemas anteriores
-- y pueden seguir presentes en DBs antiguas. Ya no se usan; no se borran para
-- no tocar datos existentes sin necesidad. epg_entries se retiró al revertir la
-- guía de programación: la única fuente XMLTV pública con ids compatibles
-- cubría 465 canales de India de 477, inservible para este catálogo. epg_programmes
-- (arriba) es la tabla nueva de P2, distinta y sin relación con aquella.
