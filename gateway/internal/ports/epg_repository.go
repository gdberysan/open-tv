package ports

import (
	"context"
	"time"

	"github.com/tu-org/iptv-ecosystem/gateway/internal/domain"
)

type EPGRepository interface {
	// SaveBatch persiste entradas parseadas de XMLTV. El ChannelID de cada
	// entrada llega siendo el tvg-id del XMLTV (así lo emite el parser); la
	// implementación lo expande a todos los canales cuyo tvg_id coincida.
	// Entradas cuyo tvg-id no matchea ningún canal se descartan en silencio.
	SaveBatch(ctx context.Context, entries []domain.EPGEntry) error

	// FindByChannelAndWindow devuelve la programación de un canal (ID propio,
	// no tvg-id) que solapa la ventana [from, to), ordenada por inicio.
	FindByChannelAndWindow(ctx context.Context, channelID domain.ChannelID, from, to time.Time) ([]domain.EPGEntry, error)

	// FindCurrentlyAiring devuelve, para todos los canales, los programas
	// en emisión en el instante dado.
	FindCurrentlyAiring(ctx context.Context, now time.Time) ([]domain.EPGEntry, error)

	// DeleteEndedBefore purga programas terminados antes de t. Devuelve
	// cuántas filas eliminó.
	DeleteEndedBefore(ctx context.Context, t time.Time) (int64, error)
}
