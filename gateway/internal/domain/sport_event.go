package domain

import "time"

// EventStatus representa el estado actual de un evento deportivo.
type EventStatus string

const (
	EventStatusScheduled EventStatus = "scheduled"
	EventStatusLive      EventStatus = "live"
	EventStatusFinished  EventStatus = "finished"
	EventStatusCancelled EventStatus = "cancelled"
)

// SportCategory agrupa el tipo de deporte.
type SportCategory string

const (
	SportFootball   SportCategory = "football"
	SportBasketball SportCategory = "basketball"
	SportTennis     SportCategory = "tennis"
	SportF1         SportCategory = "formula1"
	SportBoxing     SportCategory = "boxing"
	SportOther      SportCategory = "other"
)

// SportEvent modela un evento deportivo en vivo con múltiples mirrors.
type SportEvent struct {
	ID          string
	Title       string
	Category    SportCategory
	Status      EventStatus
	StartsAt    time.Time
	EndsAt      time.Time
	ChannelName string        // Canal que transmite (ej: "ESPN HD")
	Mirrors     []LiveStreamMirror
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// BestMirror devuelve el mirror más saludable según score compuesto.
// Prioriza: is_alive → menor latencia → menor error_rate → mayor bitrate.
func (e *SportEvent) BestMirror() (LiveStreamMirror, bool) {
	var best LiveStreamMirror
	found := false

	for _, m := range e.Mirrors {
		if !m.IsAlive {
			continue
		}
		if !found {
			best = m
			found = true
			continue
		}
		// Comparación compuesta: menor latencia primero, luego mayor bitrate
		if m.Health.LatencyMs < best.Health.LatencyMs ||
			(m.Health.LatencyMs == best.Health.LatencyMs && m.Health.BitrateKbps > best.Health.BitrateKbps) {
			best = m
		}
	}

	return best, found
}

// IsLive indica si el evento se está emitiendo ahora mismo.
func (e *SportEvent) IsLive() bool {
	return e.Status == EventStatusLive
}
