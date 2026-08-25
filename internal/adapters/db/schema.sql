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
CREATE TABLE IF NOT EXISTS providers (
    id           TEXT    PRIMARY KEY,
    type         TEXT    NOT NULL CHECK (type IN ('opensource')),
    base_url     TEXT    NOT NULL,
    credentials  TEXT,
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

-- Nota: sync_log, user_agents y epg_entries existieron en esquemas anteriores
-- y pueden seguir presentes en DBs antiguas. Ya no se usan; no se borran para
-- no tocar datos existentes sin necesidad. epg_entries se retiró al revertir la
-- guía de programación: la única fuente XMLTV pública con ids compatibles
-- cubría 465 canales de India de 477, inservible para este catálogo.
