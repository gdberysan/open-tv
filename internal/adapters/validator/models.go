package validator

import (
	"time"

	"github.com/gdberysan/open-tv/internal/domain"
)

// StreamResult representa el resultado de una validación de stream.
type StreamResult struct {
	URL       string
	IsAlive   bool
	LatencyMs int64
	Protocol  string
	Error     error

	// Compatibilidad con AirPlay, cuando se ha podido determinar. Solo la
	// rellena el camino con fallback a GET: es el único que lee cuerpo.
	Airplay domain.AirplaySupport

	// Reproducibilidad directa en un navegador. Sale del mismo GET que decide
	// IsAlive: esquema final, ACAO de la respuesta y CODECS del manifiesto.
	// Cero valor = WebUnknown = no se pudo preguntar.
	Web domain.WebSupport
}

// Config estructura las configuraciones del Validator.
type Config struct {
	// MaxWorkers define la cantidad máxima de goroutines concurrentes. Default: 50.
	MaxWorkers int
	// Timeout define el tiempo de espera máximo por petición HTTP. Default: 8s.
	Timeout time.Duration
}

// DefaultConfig devuelve la configuración por defecto recomendada para el validador.
func DefaultConfig() Config {
	return Config{
		MaxWorkers: 50,
		Timeout:    8 * time.Second,
	}
}
