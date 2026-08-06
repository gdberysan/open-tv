package failover

import (
	"context"
	"log/slog"
	"sort"
	"sync"

	"github.com/tu-org/iptv-ecosystem/gateway/internal/domain"
)

// Manager gestiona la selección y conmutación transparente de mirrors
// para eventos deportivos, asegurando continuidad durante caídas.
type Manager struct {
	mu      sync.RWMutex
	mirrors map[string][]domain.LiveStreamMirror // eventID → mirrors ordenados por prioridad
	logger  *slog.Logger
}

// NewManager crea un nuevo gestor de failover.
func NewManager(logger *slog.Logger) *Manager {
	return &Manager{
		mirrors: make(map[string][]domain.LiveStreamMirror),
		logger:  logger,
	}
}

// RegisterMirrors registra o reemplaza los mirrors para un evento.
// Los mirrors se ordenan internamente por prioridad ascendente.
func (m *Manager) RegisterMirrors(eventID string, mirrors []domain.LiveStreamMirror) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Clonar para evitar mutación externa
	cloned := make([]domain.LiveStreamMirror, len(mirrors))
	copy(cloned, mirrors)

	// Ordenar por prioridad (menor = más preferido)
	sort.Slice(cloned, func(i, j int) bool {
		return cloned[i].Priority < cloned[j].Priority
	})

	m.mirrors[eventID] = cloned

	m.logger.Info("failover: mirrors registrados",
		slog.String("event_id", eventID),
		slog.Int("count", len(cloned)),
	)
}

// BestMirror devuelve el mirror más saludable para un evento dado.
// Thread-safe para lecturas concurrentes desde múltiples handlers HTTP.
func (m *Manager) BestMirror(ctx context.Context, eventID string) (domain.LiveStreamMirror, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	mirrors, exists := m.mirrors[eventID]
	if !exists || len(mirrors) == 0 {
		return domain.LiveStreamMirror{}, false
	}

	// Primer mirror vivo no degradado
	for _, mirror := range mirrors {
		if mirror.IsAlive && !mirror.Health.IsDegraded() {
			return mirror, true
		}
	}

	// Fallback: primer mirror vivo aunque esté degradado
	for _, mirror := range mirrors {
		if mirror.IsAlive {
			m.logger.Warn("failover: solo mirrors degradados disponibles",
				slog.String("event_id", eventID),
				slog.String("mirror_id", mirror.ID),
				slog.Float64("buffer_ratio", mirror.Health.BufferRatio),
			)
			return mirror, true
		}
	}

	m.logger.Error("failover: todos los mirrors caídos",
		slog.String("event_id", eventID),
	)
	return domain.LiveStreamMirror{}, false
}

// MarkDead marca un mirror como caído y activa el failover al siguiente.
// Llamado por el health-checker o cuando el proxy detecta un error de conexión.
func (m *Manager) MarkDead(eventID, mirrorID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	mirrors, exists := m.mirrors[eventID]
	if !exists {
		return
	}

	for i := range mirrors {
		if mirrors[i].ID == mirrorID {
			mirrors[i].IsAlive = false
			m.logger.Warn("failover: mirror marcado como caído — conmutando",
				slog.String("event_id", eventID),
				slog.String("mirror_id", mirrorID),
			)
			break
		}
	}
}

// MarkAlive marca un mirror como disponible con sus métricas actualizadas.
func (m *Manager) MarkAlive(eventID, mirrorID string, health domain.MirrorHealth) {
	m.mu.Lock()
	defer m.mu.Unlock()

	mirrors, exists := m.mirrors[eventID]
	if !exists {
		return
	}

	for i := range mirrors {
		if mirrors[i].ID == mirrorID {
			mirrors[i].IsAlive = true
			mirrors[i].Health = health
			break
		}
	}
}

// ListMirrors devuelve la lista actual de mirrors para un evento (solo lectura).
func (m *Manager) ListMirrors(eventID string) []domain.LiveStreamMirror {
	m.mu.RLock()
	defer m.mu.RUnlock()

	mirrors := m.mirrors[eventID]
	result := make([]domain.LiveStreamMirror, len(mirrors))
	copy(result, mirrors)
	return result
}

// AliveCount devuelve cuántos mirrors están activos y saludables para un evento.
func (m *Manager) AliveCount(eventID string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, m := range m.mirrors[eventID] {
		if m.IsAlive && !m.Health.IsDegraded() {
			count++
		}
	}
	return count
}
