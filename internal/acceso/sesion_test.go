package acceso_test

import (
	"strings"
	"testing"
	"time"

	"github.com/gdberysan/open-tv/internal/acceso"
)

func TestClaveCorrecta(t *testing.T) {
	s := acceso.NuevasSesiones("la-clave", nil)
	if !s.ClaveCorrecta("la-clave") {
		t.Error("la clave buena no entra")
	}
	for _, mala := range []string{"", "la-clav", "la-clave ", "LA-CLAVE"} {
		if s.ClaveCorrecta(mala) {
			t.Errorf("%q entra", mala)
		}
	}
}

func TestSesionEmitidaValidaHastaQueCaduca(t *testing.T) {
	ahora := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	s := acceso.NuevasSesiones("la-clave", func() time.Time { return ahora })

	token, err := s.Emitir()
	if err != nil {
		t.Fatalf("Emitir: %v", err)
	}
	if !s.Valida(token) {
		t.Fatal("el token recién emitido no valida")
	}

	ahora = ahora.Add(acceso.DuracionSesion - time.Minute)
	if !s.Valida(token) {
		t.Error("caducó antes de tiempo")
	}
	ahora = ahora.Add(2 * time.Minute)
	if s.Valida(token) {
		t.Error("sigue valiendo después de caducar")
	}
}

func TestSesionNoSobreviveACambiarLaClave(t *testing.T) {
	token, err := acceso.NuevasSesiones("vieja", nil).Emitir()
	if err != nil {
		t.Fatalf("Emitir: %v", err)
	}
	if acceso.NuevasSesiones("nueva", nil).Valida(token) {
		t.Error("una sesión emitida con la clave vieja vale con la nueva")
	}
}

func TestSesionRechazaTokensManipulados(t *testing.T) {
	s := acceso.NuevasSesiones("la-clave", nil)
	token, err := s.Emitir()
	if err != nil {
		t.Fatalf("Emitir: %v", err)
	}
	partes := strings.Split(token, ".")
	if len(partes) != 4 {
		t.Fatalf("token %q: quiero 4 partes", token)
	}
	lejos := strings.Join([]string{partes[0], "99999999999", partes[2], partes[3]}, ".")
	for _, malo := range []string{"", "v1", "v1.1.2", token + "x", lejos, "v2." + strings.Join(partes[1:], ".")} {
		if s.Valida(malo) {
			t.Errorf("%q valida", malo)
		}
	}
}
