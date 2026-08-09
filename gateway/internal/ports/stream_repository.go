package ports

import (
	"context"

	"github.com/gdberysan/open-tv/gateway/internal/domain"
)

// StreamHealth es el resultado de un chequeo de salud listo para persistir.
type StreamHealth struct {
	StreamID  string
	IsAlive   bool
	LatencyMs int64
}

type StreamRepository interface {
	Save(ctx context.Context, s domain.Stream) error
	SaveBatch(ctx context.Context, streams []domain.Stream) error
	// FindAll devuelve el catálogo completo; lo consume el health-worker.
	FindAll(ctx context.Context) ([]domain.Stream, error)
	FindByChannelID(ctx context.Context, channelID domain.ChannelID) ([]domain.Stream, error)
	FindBestByChannelID(ctx context.Context, channelID domain.ChannelID) (domain.Stream, error) // menor latencia, is_alive=true
	MarkAlive(ctx context.Context, streamID string, latencyMs int64) error
	MarkDead(ctx context.Context, streamID string) error
	// MarkBatch aplica todos los resultados en una sola transacción. El worker
	// chequea ~12k streams por pasada: hacerlo con un UPDATE suelto por stream
	// son ~12k transacciones implícitas y otros tantos fsync, y con
	// MaxOpenConns(1) ese tiempo es API congelada.
	MarkBatch(ctx context.Context, resultados []StreamHealth) error
}
