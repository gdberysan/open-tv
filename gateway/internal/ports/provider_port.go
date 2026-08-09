package ports

import (
	"context"

	"github.com/gdberysan/open-tv/gateway/internal/domain"
)

// ProviderPort es el contrato que toda fuente de datos debe cumplir.
// Las implementaciones viven en adapters/providers/.
type ProviderPort interface {
	ID() string
	Type() domain.ProviderType

	// GetLiveChannels recupera el catálogo completo de canales en vivo.
	// Debe ser idempotente y tolerante a fallos parciales.
	GetLiveChannels(ctx context.Context) ([]domain.Channel, error)

	// GetStreamURL resuelve la URL de reproducción final para un canal.
	// Gestiona tokens dinámicos, cookies y rotación de URLs internamente.
	GetStreamURL(ctx context.Context, channelID domain.ChannelID) (string, error)

	// HealthCheck verifica conectividad con la fuente. Timeout recomendado: 5s.
	HealthCheck(ctx context.Context) error
}
