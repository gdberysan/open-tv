package domain

import "time"

// Regla «sin imagen desde aquí» (spec tiempo-hasta-la-imagen §3.3): misma
// filosofía que la histéresis del health-check (3 chequeos para declarar
// muerto). Dos fallos REALES consecutivos dentro de 24 h y el mirror se
// salta; pasada la ventana vuelve a tener una oportunidad.
const (
	UmbralFallosReales  = 2
	VentanaFallosReales = 24 * time.Hour
)

// SinImagen se evalúa en el SERVIDOR y viaja decidida en el cable: el
// cliente nunca rederiva la regla. Una fila sin fecha de desenlace nunca se
// salta, tenga los fallos que tenga: sin fecha no hay ventana.
func SinImagen(fallosReales int, ultimoDesenlace, ahora time.Time) bool {
	if fallosReales < UmbralFallosReales || ultimoDesenlace.IsZero() {
		return false
	}
	return ahora.Sub(ultimoDesenlace) <= VentanaFallosReales
}

// MotivoEsFalloReal separa «el origen no dio imagen desde aquí» de lo que no
// es culpa del origen o ya tiene su propio veredicto: geo (es dónde estamos),
// formato y codec (veredictos propios), sinImagen (no hubo intento).
func MotivoEsFalloReal(motivo string) bool {
	switch motivo {
	case "desconocido", "inestable", "caido", "caducado":
		return true
	}
	return false
}
