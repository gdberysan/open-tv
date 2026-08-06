package domain

import "time"

// MirrorHealth almacena las métricas de salud de un mirror en tiempo real.
type MirrorHealth struct {
	BitrateKbps  int64   // Bitrate medido del segmento .ts en Kbps
	LatencyMs    int64   // Latencia al descargar el primer segmento
	JitterMs     int64   // Varianza de latencia entre segmentos (predictor de buffering)
	ErrorRate    float64 // Porcentaje de fallos en los últimos N checks (0.0–1.0)
	BufferRatio  float64 // download_time / segment_duration (>0.8 = degradado)
	LastChecked  time.Time
}

// IsDegraded evalúa si un mirror está en proceso de caída inminente.
// Buffer ratio >80% es el indicador SOTA para predicción de caída.
func (h MirrorHealth) IsDegraded() bool {
	return h.BufferRatio > 0.80 || h.ErrorRate > 0.30
}

// LiveStreamMirror representa una URL de stream con sus headers de autenticación
// y sus métricas de salud actuales.
type LiveStreamMirror struct {
	ID         string
	EventID    string
	URL        string
	Protocol   Protocol
	Priority   int // Menor = mayor prioridad
	IsAlive    bool

	// Headers HTTP requeridos por el servidor origen (Referer, Cookie, etc.)
	// Se serializan como mapa para inyección dinámica en el proxy.
	RequiredHeaders map[string]string

	Health    MirrorHealth
	CreatedAt time.Time
}
