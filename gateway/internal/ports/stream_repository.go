package ports

import (
	"context"

	"github.com/tu-org/iptv-ecosystem/gateway/internal/domain"
)

type StreamRepository interface {
	Save(ctx context.Context, s domain.Stream) error
	SaveBatch(ctx context.Context, streams []domain.Stream) error
	// FindAll devuelve el catálogo completo; lo consume el health-worker.
	FindAll(ctx context.Context) ([]domain.Stream, error)
	FindByChannelID(ctx context.Context, channelID domain.ChannelID) ([]domain.Stream, error)
	FindBestByChannelID(ctx context.Context, channelID domain.ChannelID) (domain.Stream, error) // menor latencia, is_alive=true
	MarkAlive(ctx context.Context, streamID string, latencyMs int64) error
	MarkDead(ctx context.Context, streamID string) error
}
