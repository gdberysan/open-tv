package db_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/tu-org/iptv-ecosystem/gateway/internal/adapters/db"
)

// Los PRAGMAs son por conexión. Si se aplican con db.Exec sobre el pool solo
// valen para la conexión que el pool entregue en ese momento; cualquier
// conexión nueva arranca con foreign_keys = OFF y las cascadas dejan de
// aplicarse en silencio.
func TestOpenAplicaPragmasEnTodasLasConexiones(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	sqlDB, err := db.Open(path)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer sqlDB.Close()

	sqlDB.SetMaxOpenConns(4)

	ctx := context.Background()
	conns := make([]*sql.Conn, 0, 4)
	for i := 0; i < 4; i++ {
		c, err := sqlDB.Conn(ctx)
		if err != nil {
			t.Fatalf("Conn(%d): %v", i, err)
		}
		conns = append(conns, c)
	}
	defer func() {
		for _, c := range conns {
			c.Close()
		}
	}()

	for i, c := range conns {
		var fk int
		if err := c.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&fk); err != nil {
			t.Fatalf("conexión %d: PRAGMA foreign_keys: %v", i, err)
		}
		if fk != 1 {
			t.Errorf("conexión %d: foreign_keys = %d, quiero 1", i, fk)
		}

		var busy int
		if err := c.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&busy); err != nil {
			t.Fatalf("conexión %d: PRAGMA busy_timeout: %v", i, err)
		}
		if busy < 5000 {
			t.Errorf("conexión %d: busy_timeout = %d, quiero >= 5000", i, busy)
		}
	}
}

// migrate() trocea schema.sql por ";". Un ";" dentro de un comentario partía la
// sentencia y dejaba un fragmento de prosa que SQLite intentaba ejecutar; pasó
// de verdad al documentar unas tablas obsoletas.
func TestMigrateToleraPuntoYComaEnComentarios(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	sqlDB, err := db.Open(path)
	if err != nil {
		t.Fatalf("db.Open con comentarios en el esquema: %v", err)
	}
	defer sqlDB.Close()

	// Si el troceo hubiera fallado, faltarían tablas.
	for _, tabla := range []string{"channels", "streams", "epg_entries", "providers", "categories"} {
		var n int
		if err := sqlDB.QueryRow(
			"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", tabla,
		).Scan(&n); err != nil {
			t.Fatalf("consultando sqlite_master: %v", err)
		}
		if n != 1 {
			t.Errorf("falta la tabla %q tras migrar", tabla)
		}
	}
}
