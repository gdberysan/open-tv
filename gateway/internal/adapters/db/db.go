package db

import (
	"database/sql"
	"embed"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaFS embed.FS

// Open abre o crea la base de datos SQLite, aplica PRAGMAs, migra el esquema
// y siembra los providers por defecto. Listo para usar tras retornar.
func Open(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("db.Open: %w", err)
	}

	// WAL permite lecturas concurrentes, pero SQLite serializa escrituras
	// internamente. Una sola conexión abierta evita errores SQLITE_BUSY en MVP.
	db.SetMaxOpenConns(1)

	if err := applyPragmas(db); err != nil {
		db.Close()
		return nil, err
	}
	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}
	if err := seedProviders(db); err != nil {
		db.Close()
		return nil, err
	}
	if err := seedUserAgents(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func applyPragmas(db *sql.DB) error {
	for _, p := range []string{
		"PRAGMA journal_mode = WAL",
		"PRAGMA synchronous   = NORMAL",
		"PRAGMA cache_size    = -32000",
		"PRAGMA foreign_keys  = ON",
	} {
		if _, err := db.Exec(p); err != nil {
			return fmt.Errorf("db.applyPragmas (%s): %w", p, err)
		}
	}
	return nil
}

// migrate ejecuta el esquema sentencia por sentencia.
// database/sql no garantiza ejecución de múltiples statements en un solo Exec().
func migrate(db *sql.DB) error {
	schema, err := schemaFS.ReadFile("schema.sql")
	if err != nil {
		return fmt.Errorf("db.migrate (ReadFile): %w", err)
	}
	for _, stmt := range strings.Split(string(schema), ";") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := db.Exec(stmt); err != nil {
			short := stmt
			if len(short) > 60 {
				short = short[:60]
			}
			return fmt.Errorf("db.migrate (%s...): %w", short, err)
		}
	}
	return nil
}

// seedUserAgents inserta el pool inicial de User-Agents para el health-checker.
// Se mantiene en Go (no en schema.sql) para evitar conflictos con ";" dentro de strings.
func seedUserAgents(db *sql.DB) error {
	agents := []struct{ id, ua, typ string }{
		{"ua-1", "VLC/3.0.20 LibVLC/3.0.20", "media_player"},
		{"ua-2", "ExoPlayer/2.18.0 (Linux; Android 13)", "media_player"},
		{"ua-3", "Lavf/58.76.100", "media_player"},
		{"ua-4", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36", "browser"},
		{"ua-5", "HLS.js/1.4.0 (env: production)", "media_player"},
	}
	for _, a := range agents {
		if _, err := db.Exec(
			`INSERT OR IGNORE INTO user_agents (id, ua_string, ua_type) VALUES (?, ?, ?)`,
			a.id, a.ua, a.typ,
		); err != nil {
			return fmt.Errorf("db.seedUserAgents (%s): %w", a.id, err)
		}
	}
	return nil
}

// seedProviders inserta el provider IPTV-org si no existe aún.
func seedProviders(db *sql.DB) error {
	now := time.Now().Unix()
	seeds := []struct{ id, typ, baseURL string }{
		{"opensource", "opensource", "https://iptv-org.github.io/iptv/index.m3u"},
	}
	for _, s := range seeds {
		if _, err := db.Exec(`
			INSERT INTO providers (id, type, base_url, priority, is_active, created_at, updated_at)
			VALUES (?, ?, ?, 100, 1, ?, ?)
			ON CONFLICT(id) DO NOTHING
		`, s.id, s.typ, s.baseURL, now, now); err != nil {
			return fmt.Errorf("db.seedProviders (%s): %w", s.id, err)
		}
	}
	return nil
}
