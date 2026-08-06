package ports

import (
	"context"
	"strings"

	"github.com/tu-org/iptv-ecosystem/gateway/internal/domain"
)

// ChannelFilter agrupa los parámetros de filtrado para FindFiltered.
type ChannelFilter struct {
	Query      string // búsqueda por nombre (LIKE)
	Country    string // ISO 3166-1 alpha-2
	Category   string // ID exacto de categoría
	MinQuality string // "4k" | "fhd" (1080p+) | "hd" (720p+) | "" (todos)
	// AliveOnly oculta canales cuyos streams fueron TODOS chequeados y están
	// TODOS muertos. Canales sin chequear (o sin streams) siguen visibles:
	// antes de la primera pasada del health-worker nada está "vivo" aún.
	AliveOnly bool
	Limit     int
	Offset    int
}

// Normalize estandariza los campos del filtro antes de usarlo.
func (f ChannelFilter) Normalize() ChannelFilter {
	f.Country = strings.ToUpper(strings.TrimSpace(f.Country))
	f.MinQuality = strings.ToLower(strings.TrimSpace(f.MinQuality))
	if f.Limit <= 0 {
		f.Limit = 500
	}
	if f.Limit > 1000 {
		f.Limit = 1000
	}
	return f
}

type ChannelRepository interface {
	Save(ctx context.Context, ch domain.Channel) error
	SaveBatch(ctx context.Context, channels []domain.Channel) error
	FindByID(ctx context.Context, id domain.ChannelID) (domain.Channel, error)
	FindFiltered(ctx context.Context, f ChannelFilter) ([]domain.Channel, error)
	Search(ctx context.Context, query string, limit int) ([]domain.Channel, error)
	Delete(ctx context.Context, id domain.ChannelID) error
}
