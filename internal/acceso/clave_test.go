package acceso_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/gdberysan/open-tv/internal/acceso"
)

func TestCargarOGenerarPrefiereLaVariableDeEntorno(t *testing.T) {
	dir := t.TempDir()
	clave, generada, err := acceso.CargarOGenerar(dir, "  desde-el-entorno  ")
	if err != nil {
		t.Fatalf("CargarOGenerar: %v", err)
	}
	if clave != "desde-el-entorno" || generada {
		t.Errorf("clave=%q generada=%v, quiero la del entorno sin espacios y sin generar", clave, generada)
	}
	if _, err := os.Stat(filepath.Join(dir, acceso.FicheroClave)); !os.IsNotExist(err) {
		t.Error("con la clave en el entorno no debe escribirse fichero")
	}
}

func TestCargarOGenerarCreaUnaVezYLuegoReutiliza(t *testing.T) {
	dir := t.TempDir()
	primera, generada, err := acceso.CargarOGenerar(dir, "")
	if err != nil {
		t.Fatalf("primera: %v", err)
	}
	if !generada || len(primera) < 40 {
		t.Fatalf("primera=%q generada=%v: quiero una clave nueva de >= 40 caracteres", primera, generada)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(filepath.Join(dir, acceso.FicheroClave))
		if err != nil {
			t.Fatalf("Stat: %v", err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Errorf("permisos %v, quiero 0600", info.Mode().Perm())
		}
	}

	segunda, generada, err := acceso.CargarOGenerar(dir, "")
	if err != nil {
		t.Fatalf("segunda: %v", err)
	}
	if generada || segunda != primera {
		t.Errorf("segunda=%q generada=%v, quiero la misma clave sin regenerar", segunda, generada)
	}
}

func TestCargarOGenerarRechazaFicheroVacio(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, acceso.FicheroClave), []byte("\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := acceso.CargarOGenerar(dir, ""); err == nil {
		t.Error("un fichero de clave vacío se aceptó: nadie podría entrar")
	}
}
