package acceso_test

import (
	"testing"
	"time"

	"github.com/gdberysan/open-tv/internal/acceso"
)

func TestLimitadorCortaTrasElMaximoYSeRecupera(t *testing.T) {
	ahora := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	l := acceso.NuevoLimitador(3, time.Minute, func() time.Time { return ahora })

	for i := 0; i < 3; i++ {
		if !l.Permitir("10.0.0.2") {
			t.Fatalf("intento %d cortado antes del máximo", i+1)
		}
	}
	if l.Permitir("10.0.0.2") {
		t.Error("el cuarto intento en la misma ventana pasó")
	}
	if !l.Permitir("10.0.0.3") {
		t.Error("otra IP paga los intentos de la primera")
	}

	ahora = ahora.Add(time.Minute + time.Second)
	if !l.Permitir("10.0.0.2") {
		t.Error("no se recupera al pasar la ventana")
	}
}
