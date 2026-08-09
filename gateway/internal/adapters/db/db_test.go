package db_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/gdberysan/open-tv/gateway/internal/adapters/db"
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
	for _, tabla := range []string{"channels", "streams", "providers", "categories"} {
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

// WAL existe para que los lectores no esperen al escritor. Con un único pool de
// una conexión, cualquier lectura se encola detrás de la escritura en curso:
// medido, 760ms de espera tras una transacción de 800ms.
func TestLecturaNoSeBloqueaDetrasDeUnaEscritura(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")

	escritura, err := db.Open(path)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer escritura.Close()

	lectura, err := db.OpenReadOnly(path)
	if err != nil {
		t.Fatalf("db.OpenReadOnly: %v", err)
	}
	defer lectura.Close()

	ctx := context.Background()
	tx, err := escritura.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("BeginTx: %v", err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO channels (id, name, provider_id, provider_type, is_adult, created_at, updated_at, last_seen_at)
		 VALUES ('x', 'X', 'opensource', 'opensource', 0, 0, 0, 0)`); err != nil {
		t.Fatalf("Exec: %v", err)
	}

	inicio := time.Now()
	var n int
	if err := lectura.QueryRowContext(ctx, "SELECT COUNT(*) FROM channels").Scan(&n); err != nil {
		t.Fatalf("lectura concurrente: %v", err)
	}
	transcurrido := time.Since(inicio)

	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	if transcurrido > 200*time.Millisecond {
		t.Errorf("la lectura tardó %v con una escritura abierta; WAL debería permitirla sin esperar", transcurrido)
	}
}

// El pool de lectura no debe poder escribir: si un handler intentase un UPDATE
// por error, mejor que falle en desarrollo que corromper el orden de escrituras.
func TestOpenReadOnlyRechazaEscrituras(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	escritura, err := db.Open(path)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer escritura.Close()

	lectura, err := db.OpenReadOnly(path)
	if err != nil {
		t.Fatalf("db.OpenReadOnly: %v", err)
	}
	defer lectura.Close()

	_, err = lectura.Exec(`INSERT INTO providers (id, type, base_url, priority, is_active, created_at, updated_at)
	                       VALUES ('x','opensource','http://x',1,1,0,0)`)
	if err == nil {
		t.Error("el pool de solo lectura no debería aceptar escrituras")
	}
}
