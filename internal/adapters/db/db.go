package db

import (
	"database/sql"
	"embed"
	"fmt"
	"strings"

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

// Open abre o crea la base de datos SQLite y migra el esquema. Listo para
// usar tras retornar. Una instalación limpia arranca con CERO providers: ya
// no hay seed de IPTV-org — el catálogo depende de las fuentes que el usuario
// dé de alta con SourceRepository (ver source_repository.go).
func Open(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", "file:"+path+"?"+pragmaDSN)
	if err != nil {
		return nil, fmt.Errorf("db.Open: %w", err)
	}

	// WAL permite lecturas concurrentes, pero SQLite serializa escrituras
	// internamente. Una sola conexión abierta evita errores SQLITE_BUSY en MVP.
	db.SetMaxOpenConns(1)

	if err := migrate(db); err != nil {
		_ = db.Close()
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
	for _, stmt := range strings.Split(stripSQLComments(string(schema)), ";") {
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

// stripSQLComments quita los comentarios de línea antes de trocear el esquema
// por ";". Sin esto, un ";" dentro de un comentario —p.ej. "(nunca se
// escribieron ni se leyeron); ..."— parte la sentencia por la mitad y deja un
// fragmento de prosa que SQLite intenta ejecutar. Pasó de verdad.
//
// Limitación conocida: no distingue un "--" dentro de un literal de cadena.
// El esquema no tiene ninguno; si algún día lo tiene, hará falta un troceador
// de sentencias en condiciones.
func stripSQLComments(s string) string {
	var b strings.Builder
	for _, line := range strings.Split(s, "\n") {
		if i := strings.Index(line, "--"); i >= 0 {
			line = line[:i]
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}

// alterMigrations añade columnas a tablas de DBs creadas con un esquema
// anterior. Idempotente: tolera columna duplicada (ya migrada) y tabla
// inexistente (DB nueva; el CREATE TABLE posterior ya incluye la columna).
func alterMigrations(db *sql.DB) error {
	alters := []string{
		"ALTER TABLE channels ADD COLUMN tvg_id TEXT",
		"ALTER TABLE streams ADD COLUMN fail_count INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE channels ADD COLUMN last_seen_at INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE streams ADD COLUMN web_ok INTEGER",
		// Fuentes "bring your own" (retirada del seed de IPTV-org): label es
		// el nombre que el usuario le pone a la fuente; kind distingue cómo se
		// obtiene el M3U ('url' remota o 'file' subido), no el formato.
		"ALTER TABLE providers ADD COLUMN label TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE providers ADD COLUMN kind TEXT NOT NULL DEFAULT 'url'",
		// url-tvg declarada en la cabecera M3U de la fuente (ver
		// opensource.Provider.TvgURLs); '' = la fuente no la declaró.
		"ALTER TABLE providers ADD COLUMN tvg_url TEXT NOT NULL DEFAULT ''",
		// Cadencia de refresco EPG por fuente (Tarea 5 de P2); 0 = nunca
		// refrescada. Ver services.Syncer.guiaEstaFresca.
		"ALTER TABLE providers ADD COLUMN epg_refreshed_at INTEGER NOT NULL DEFAULT 0",
		// Cabeceras por stream y frontera de poda (spec de mirrors, 2026-09-04).
		"ALTER TABLE streams ADD COLUMN referrer TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE streams ADD COLUMN user_agent TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE streams ADD COLUMN last_seen_at INTEGER NOT NULL DEFAULT 0",
		// Sonda de códecs por segmento (spec salud-por-segmento, 2026-09-05).
		"ALTER TABLE streams ADD COLUMN codec_ok INTEGER",
		"ALTER TABLE streams ADD COLUMN codecs TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE streams ADD COLUMN codec_checked_at INTEGER NOT NULL DEFAULT 0",
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

// OpenReadOnly abre un pool de SOLO LECTURA sobre la misma DB. En WAL los
// lectores no bloquean al escritor ni viceversa, pero el pool de escritura está
// limitado a una conexión para evitar SQLITE_BUSY; si los handlers compartieran
// ese pool, cada lectura se encolaría detrás de la escritura en curso —medido:
// 760ms de espera tras una transacción de 800ms, y un SaveBatch de 13.8k
// canales ocupa la conexión 167ms.
//
// Asume que Open ya creó y migró la DB.
func OpenReadOnly(path string) (*sql.DB, error) {
	dsn := "file:" + path + "?mode=ro" +
		"&_pragma=busy_timeout(5000)" +
		"&_pragma=foreign_keys(1)" +
		"&_pragma=cache_size(-32000)"

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("db.OpenReadOnly: %w", err)
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("db.OpenReadOnly (Ping): %w", err)
	}
	return db, nil
}
