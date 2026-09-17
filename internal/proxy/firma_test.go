package proxy_test

import (
	"bytes"
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
