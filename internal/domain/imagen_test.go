package domain_test

import (
	"testing"
	"time"

	"github.com/gdberysan/open-tv/internal/domain"
)

func TestSinImagen(t *testing.T) {
	ahora := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	casos := []struct {
		nombre string
		fallos int
		ultimo time.Time
		quiero bool
	}{
		{"nunca falló", 0, time.Time{}, false},
		{"un fallo reciente", 1, ahora.Add(-time.Hour), false},
		{"dos fallos recientes", 2, ahora.Add(-time.Hour), true},
		{"tres fallos recientes", 3, ahora.Add(-time.Minute), true},
		{"dos fallos hace exactamente 24 h: aún dentro", 2, ahora.Add(-domain.VentanaFallosReales), true},
		{"dos fallos hace 24 h y un segundo: otra oportunidad", 2, ahora.Add(-domain.VentanaFallosReales - time.Second), false},
		{"fallos sin fecha (fila corrupta): no se salta", 5, time.Time{}, false},
	}
	for _, c := range casos {
		if got := domain.SinImagen(c.fallos, c.ultimo, ahora); got != c.quiero {
			t.Errorf("%s: SinImagen = %v, quiero %v", c.nombre, got, c.quiero)
		}
	}
}

func TestMotivoEsFalloReal(t *testing.T) {
	reales := []string{"desconocido", "inestable", "caido", "caducado"}
	noReales := []string{"geo", "formato", "codec", "sinImagen", "", "otra"}
	for _, m := range reales {
		if !domain.MotivoEsFalloReal(m) {
			t.Errorf("%q debe ser fallo real", m)
		}
	}
	for _, m := range noReales {
		if domain.MotivoEsFalloReal(m) {
			t.Errorf("%q NO debe ser fallo real", m)
		}
	}
}
