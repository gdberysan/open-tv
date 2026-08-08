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

// pragmaDSN son los PRAGMAs que toda conexión debe tener. Van en el DSN, no en
// un Exec posterior: son estado POR CONEXIÓN y el driver los reaplica en cada
// conexión que abre el pool. Con db.Exec solo se configuraría la conexión que
// el pool entregue en ese momento, y cualquier conexión nueva arrancaría con
// foreign_keys en su default (OFF), perdiendo las cascadas en silencio.
const pragmaDSN = "_pragma=journal_mode(WAL)" +
	"&_pragma=synchronous(NORMAL)" +
	"&_pragma=cache_size(-32000)" +
	"&_pragma=foreign_keys(1)" +
	"&_pragma=busy_timeout(5000)"

// Open abre o crea la base de datos SQLite, migra el esquema y siembra los
// providers por defecto. Listo para usar tras retornar.
func Open(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", "file:"+path+"?"+pragmaDSN)
	if err != nil {
		return nil, fmt.Errorf("db.Open: %w", err)
	}

	// WAL permite lecturas concurrentes, pero SQLite serializa escrituras
	// internamente. Una sola conexión abierta evita errores SQLITE_BUSY en MVP.
	db.SetMaxOpenConns(1)

	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}
	if err := seedProviders(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// migrate ejecuta el esquema sentencia por sentencia.
// database/sql no garantiza ejecución de múltiples statements en un solo Exec().
func migrate(db *sql.DB) error {
	schema, err := schemaFS.ReadFile("schema.sql")
	if err != nil {
		return fmt.Errorf("db.migrate (ReadFile): %w", err)
	}
	// Los ALTER van ANTES del esquema: CREATE TABLE IF NOT EXISTS no añade
	// columnas a DBs viejas, y los índices del esquema pueden referenciar
	// columnas nuevas que solo existen tras el ALTER.
	if err := alterMigrations(db); err != nil {
		return err
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

// alterMigrations añade columnas a tablas de DBs creadas con un esquema
// anterior. Idempotente: tolera columna duplicada (ya migrada) y tabla
// inexistente (DB nueva; el CREATE TABLE posterior ya incluye la columna).
func alterMigrations(db *sql.DB) error {
	alters := []string{
		"ALTER TABLE channels ADD COLUMN tvg_id TEXT",
		"ALTER TABLE streams ADD COLUMN fail_count INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE channels ADD COLUMN last_seen_at INTEGER NOT NULL DEFAULT 0",
	}
	for _, stmt := range alters {
		if _, err := db.Exec(stmt); err != nil {
			msg := err.Error()
			if strings.Contains(msg, "duplicate column name") ||
				strings.Contains(msg, "no such table") {
				continue
			}
			return fmt.Errorf("db.alterMigrations (%s): %w", stmt, err)
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
