package datadir_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gdberysan/open-tv/internal/datadir"
)

func TestDefaultPorSistema(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("APPDATA", filepath.Join(home, "AppData", "Roaming"))

	got, err := datadir.Default()
	if err != nil {
		t.Fatalf("Default: %v", err)
	}

	var quiero string
	switch runtime.GOOS {
	case "darwin":
		quiero = filepath.Join(home, "Library", "Application Support", "Korven Open TV")
	case "windows":
		quiero = filepath.Join(home, "AppData", "Roaming", "Korven Open TV")
	default:
		quiero = filepath.Join(home, ".local", "share", "korven-open-tv")
	}
	if got != quiero {
		t.Errorf("Default() = %q, quiero %q", got, quiero)
	}
}

func TestXDGDataHomeGanaEnLinux(t *testing.T) {
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		t.Skip("XDG_DATA_HOME solo aplica al camino por defecto")
	}
	xdg := t.TempDir()
	t.Setenv("XDG_DATA_HOME", xdg)

	got, err := datadir.Default()
	if err != nil {
		t.Fatalf("Default: %v", err)
	}
	if got != filepath.Join(xdg, "korven-open-tv") {
		t.Errorf("Default() = %q, quiero %q", got, filepath.Join(xdg, "korven-open-tv"))
	}
}

// DB_PATH es la vía de escape documentada y sigue mandando: la usan el
// LaunchAgent, los tests del stack completo y quien quiera dos catálogos.
func TestRutaDBRespetaDBPath(t *testing.T) {
	explicita := filepath.Join(t.TempDir(), "mia.db")
	t.Setenv("DB_PATH", explicita)

	got, err := datadir.RutaDB()
	if err != nil {
		t.Fatalf("RutaDB: %v", err)
	}
	if got != explicita {
		t.Errorf("RutaDB() = %q, quiero %q", got, explicita)
	}
}

// Sin DB_PATH, RutaDB tiene que DEJAR EL DIRECTORIO CREADO: sqlite no crea
// directorios intermedios y el fallo llega como "unable to open database file",
// que no dice nada del directorio que falta.
func TestRutaDBCreaElDirectorio(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("DB_PATH", "")
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "datos"))
	t.Setenv("APPDATA", filepath.Join(home, "AppData", "Roaming"))

	got, err := datadir.RutaDB()
	if err != nil {
		t.Fatalf("RutaDB: %v", err)
	}
	if !strings.HasSuffix(got, "iptv.db") {
		t.Errorf("RutaDB() = %q, quiero que termine en iptv.db", got)
	}
	if _, err := filepath.Abs(got); err != nil {
		t.Fatalf("ruta no absoluta: %v", err)
	}
	dir := filepath.Dir(got)
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		t.Errorf("el directorio %q no quedó creado (err=%v)", dir, err)
	}
}
