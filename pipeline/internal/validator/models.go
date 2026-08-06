package validator

import "time"

// StreamResult representa el resultado de una validación de stream.
type StreamResult struct {
	URL       string
	IsAlive   bool
	LatencyMs int64
	Protocol  string
	Error     error
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
