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
    last_checked INTEGER,
    created_at   INTEGER NOT NULL,
    updated_at   INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_streams_channel_alive ON streams(channel_id, is_alive);
CREATE INDEX IF NOT EXISTS idx_streams_latency       ON streams(channel_id, latency_ms) WHERE is_alive = 1;

-- ─────────────────────────────────────────
-- EPG / GUÍA DE PROGRAMACIÓN
-- ─────────────────────────────────────────
CREATE TABLE IF NOT EXISTS epg_entries (
    id          TEXT    PRIMARY KEY,
    channel_id  TEXT    NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    title       TEXT    NOT NULL,
    description TEXT,
    start_at    INTEGER NOT NULL,
    end_at      INTEGER NOT NULL,
    created_at  INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_epg_channel_time ON epg_entries(channel_id, start_at, end_at);
CREATE INDEX IF NOT EXISTS idx_epg_window       ON epg_entries(start_at, end_at);

-- ─────────────────────────────────────────
-- AUDITORÍA DE SINCRONIZACIÓN
-- ─────────────────────────────────────────
CREATE TABLE IF NOT EXISTS sync_log (
    id              TEXT    PRIMARY KEY,
    provider_id     TEXT    NOT NULL REFERENCES providers(id) ON DELETE CASCADE,
    started_at      INTEGER NOT NULL,
    finished_at     INTEGER,
    channels_found  INTEGER DEFAULT 0,
    streams_checked INTEGER DEFAULT 0,
    streams_alive   INTEGER DEFAULT 0,
    error_message   TEXT,
    status          TEXT    NOT NULL CHECK (status IN ('running','success','partial','failed'))
);

CREATE INDEX IF NOT EXISTS idx_sync_log_provider ON sync_log(provider_id, started_at DESC);

-- ─────────────────────────────────────────
-- POOL DE USER-AGENTS ROTATIVOS
-- ─────────────────────────────────────────
CREATE TABLE IF NOT EXISTS user_agents (
    id          TEXT    PRIMARY KEY,
    ua_string   TEXT    NOT NULL UNIQUE,
    ua_type     TEXT    NOT NULL CHECK (ua_type IN ('browser','media_player','bot')),
    is_active   INTEGER NOT NULL DEFAULT 1 CHECK (is_active IN (0,1)),
    last_used   INTEGER,
    use_count   INTEGER NOT NULL DEFAULT 0
);
