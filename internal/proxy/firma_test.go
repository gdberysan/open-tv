package proxy_test

import (
	"bytes"
	"encoding/base64"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/gdberysan/open-tv/internal/proxy"
)

func firmadorDePrueba(t *testing.T) *proxy.Firmador {
	t.Helper()
	f, err := proxy.NuevoFirmador(bytes.Repeat([]byte{7}, 32))
	if err != nil {
		t.Fatalf("NuevoFirmador: %v", err)
	}
	return f
}

func TestFirmadorValidaSoloSuPropiaFirma(t *testing.T) {
	f := firmadorDePrueba(t)
	u := "https://origen.example/live/seg1.ts"
	firma := f.Firmar(u)

	if !f.Valida(u, firma) {
		t.Fatal("la firma recién emitida no valida")
	}
	if f.Valida("https://origen.example/live/seg2.ts", firma) {
		t.Error("una firma vale para otra URL")
	}
	if f.Valida(u, firma[:len(firma)-1]+"A") && firma[len(firma)-1] != 'A' {
		t.Error("una firma manipulada valida")
	}
	if f.Valida(u, "") {
		t.Error("firma vacía valida")
	}

	otro, err := proxy.NuevoFirmador(bytes.Repeat([]byte{8}, 32))
	if err != nil {
		t.Fatalf("NuevoFirmador: %v", err)
	}
	if otro.Valida(u, firma) {
		t.Error("la firma de un proceso vale en otro con otra clave")
	}
}

func TestFirmadorNilNoValidaNada(t *testing.T) {
	var f *proxy.Firmador
	if f.Valida("https://x/y", "loquesea") {
		t.Error("un firmador nil valida")
	}
}

func TestNuevoFirmadorExigeClaveLarga(t *testing.T) {
	if _, err := proxy.NuevoFirmador(make([]byte, 31)); err == nil {
		t.Error("una clave de 31 bytes se aceptó")
	}
}

func TestFirmadorAleatorioEsDistintoCadaVez(t *testing.T) {
	a, err := proxy.NuevoFirmadorAleatorio()
	if err != nil {
		t.Fatalf("NuevoFirmadorAleatorio: %v", err)
	}
	b, err := proxy.NuevoFirmadorAleatorio()
	if err != nil {
		t.Fatalf("NuevoFirmadorAleatorio: %v", err)
	}
	if a.Firmar("https://x/y") == b.Firmar("https://x/y") {
		t.Error("dos firmadores aleatorios firman igual")
	}
}

// TestFirmadorPersistenteSobreviveAlReinicio: hls.js reintenta las URLs hijas
// firmadas que ya tiene, así que un reinicio del gateway (docker restart,
// launchd) no puede invalidarlas. Dos firmadores sobre el mismo fichero son
// el mismo proceso a efectos de firma.
func TestFirmadorPersistenteSobreviveAlReinicio(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), proxy.FicheroClaveProxy)
	a, err := proxy.NuevoFirmadorPersistente(ruta)
	if err != nil {
		t.Fatalf("primer arranque: %v", err)
	}
	b, err := proxy.NuevoFirmadorPersistente(ruta)
	if err != nil {
		t.Fatalf("segundo arranque: %v", err)
	}
	u := "https://origen.example/live/seg1.ts"
	if !b.Valida(u, a.Firmar(u)) || !a.Valida(u, b.Firmar(u)) {
		t.Error("tras reiniciar, las firmas emitidas antes ya no valen")
	}

	if runtime.GOOS != "windows" {
		info, err := os.Stat(ruta)
		if err != nil {
			t.Fatalf("stat: %v", err)
		}
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Errorf("permisos de %s = %o, quiero 600", proxy.FicheroClaveProxy, perm)
		}
	}
}

func TestFirmadorPersistenteDistintoPorInstalacion(t *testing.T) {
	a, err := proxy.NuevoFirmadorPersistente(filepath.Join(t.TempDir(), proxy.FicheroClaveProxy))
	if err != nil {
		t.Fatal(err)
	}
	b, err := proxy.NuevoFirmadorPersistente(filepath.Join(t.TempDir(), proxy.FicheroClaveProxy))
	if err != nil {
		t.Fatal(err)
	}
	if a.Firmar("https://x/y") == b.Firmar("https://x/y") {
		t.Error("dos instalaciones distintas firman igual")
	}
}

func TestFirmadorPersistenteRechazaFicheroMalo(t *testing.T) {
	for _, c := range []struct {
		nombre, contenido string
	}{
		{"corta", base64.RawURLEncoding.EncodeToString(make([]byte, 31)) + "\n"},
		{"vacía", ""},
		{"no base64", "esto no es base64!!\n"},
	} {
		ruta := filepath.Join(t.TempDir(), proxy.FicheroClaveProxy)
		if err := os.WriteFile(ruta, []byte(c.contenido), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := proxy.NuevoFirmadorPersistente(ruta); err == nil {
			t.Errorf("clave %s: se aceptó", c.nombre)
		}
	}
}

func TestFirmadorPersistenteDevuelveOtrosErroresDeLectura(t *testing.T) {
	// Un directorio en lugar del fichero: no es "no existe", así que no debe
	// generarse una clave nueva encima.
	ruta := t.TempDir()
	if _, err := proxy.NuevoFirmadorPersistente(ruta); err == nil {
		t.Error("leer un directorio como clave no dio error")
	}
}
