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

	// StatusCode es el código HTTP de la última respuesta recibida (HEAD o
	// GET, el que haya decidido IsAlive). Cero valor = no se recibió
	// respuesta (error de red, timeout o de creación de la petición).
	StatusCode int

	// Compatibilidad con AirPlay, cuando se ha podido determinar. Solo la
	// rellena el camino con fallback a GET: es el único que lee cuerpo.
	Airplay domain.AirplaySupport

	// Reproducibilidad directa en un navegador. Sale del mismo GET que decide
	// IsAlive: esquema final, ACAO de la respuesta y CODECS del manifiesto.
	// Cero valor = WebUnknown = no se pudo preguntar.
	Web domain.WebSupport
}

// TareaCheck es una URL a comprobar junto con las cabeceras que su origen
// exige. Viaja por el canal de Start porque el checker no tiene acceso al
// repositorio: quien alimenta el canal (el worker) es quien conoce
// Referrer/UserAgent de cada stream.
type TareaCheck struct {
	URL       string
	Referrer  string
	UserAgent string
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
