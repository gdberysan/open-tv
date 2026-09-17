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
		if l.Agotado("10.0.0.2") {
			t.Fatalf("fallo %d: ya se daba por agotado antes del máximo", i+1)
		}
		l.Fallo("10.0.0.2")
	}
	if !l.Agotado("10.0.0.2") {
		t.Error("tras el tercer fallo en la misma ventana debería estar agotado")
	}
	if l.Agotado("10.0.0.3") {
		t.Error("otra IP paga los fallos de la primera")
	}

	ahora = ahora.Add(time.Minute + time.Second)
	if l.Agotado("10.0.0.2") {
		t.Error("no se recupera al pasar la ventana")
	}
}
