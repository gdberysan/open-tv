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

	// Codec/Codecs: veredicto de la sonda del primer segmento (PAT/PMT).
	// CodecSondeado = la sonda corrió en este chequeo, decidiera o no.
	Codec         domain.CodecSupport
	Codecs        string
	CodecSondeado bool

	// Audio sale de la misma PMT que Codec: presencia de pista de audio.
	// Solo se rellena si la sonda corrió (CodecSondeado = true).
	Audio domain.AudioSupport
}

// TareaCheck es una URL a comprobar junto con las cabeceras que su origen
// exige. Viaja por el canal de Start porque el checker no tiene acceso al
// repositorio: quien alimenta el canal (el worker) es quien conoce
// Referrer/UserAgent de cada stream.
type TareaCheck struct {
	URL       string
	Referrer  string
	UserAgent string
	// CodecCaducado: el worker lo pone a true cuando el veredicto de códecs
	// de la fila no existe, tiene más de 24 h, o el mirror venía de muerto.
	// Solo entonces el checker sondea el segmento.
	CodecCaducado bool
}

// Config estructura las configuraciones del Validator.
type Config struct {
	// MaxWorkers define la cantidad máxima de goroutines concurrentes. Default: 50.
	MaxWorkers int
	// Timeout define el tiempo de espera máximo por petición HTTP. Default: 8s.
	Timeout time.Duration
	// PermitirDestinosPrivados solo es true en tests: httptest vive en
	// 127.0.0.1, que la guardia de la sonda bloquea en producción.
	PermitirDestinosPrivados bool
}

// DefaultConfig devuelve la configuración por defecto recomendada para el validador.
func DefaultConfig() Config {
	return Config{
		MaxWorkers: 50,
		Timeout:    8 * time.Second,
	}
}
