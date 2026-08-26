package ports

import (
	"context"

	"github.com/gdberysan/open-tv/internal/domain"
)

// EPGRepository persiste y consulta la guía de programación (EPG), SIEMPRE
// aislada por fuente: dos providers pueden traer el mismo channel_id XMLTV
// con programas distintos, y el join de lectura va por (provider_id,
// channel_id) — nunca por channel_id a secas. Ver channels.tvg_id y
// providers.tvg_url.
type EPGRepository interface {
	// ReemplazarVentana sustituye TODA la guía de ese provider por la
	// ventana nueva en una sola transacción (DELETE de lo viejo + INSERT de
	// lo nuevo): no acumula entre syncs.
	ReemplazarVentana(ctx context.Context, providerID string, programas []domain.Programa) error

	// Podar borra los programas ya terminados (stop_utc < antesDe) de
	// CUALQUIER provider. Mantenimiento periódico; independiente del sync.
	Podar(ctx context.Context, antesDe int64) error

	// AhoraDespuesPorCanales devuelve, para cada canal de la app pedido, el
	// programa en curso (start_utc <= ahora < stop_utc) y el inmediato
	// siguiente (menor start_utc > ahora) en SU fuente — el join se hace por
	// (provider_id, tvg_id) del canal, así que dos canales con el mismo
	// tvg_id pero providers distintos jamás se cruzan.
	//
	// Decisión de forma del mapa (para que el handler distinga los dos
	// casos): un channelID cuyo channels.tvg_id está vacío ("" o NULL) — sin
	// guía posible — queda AUSENTE del mapa. Un channelID CON tvg_id pero sin
	// programa que cubra `ahora` ni ninguno futuro (hueco, o guía agotada)
	// SÍ aparece en el mapa, con AhoraDespues{Ahora: nil, Siguiente: nil}.
	// Un channelID que no exista en absoluto en channels tampoco aparece
	// (no hay tvg_id que resolver).
	AhoraDespuesPorCanales(ctx context.Context, channelIDs []string, ahora int64) (map[string]domain.AhoraDespues, error)

	// ProximosDeCanal devuelve el programa en curso (si lo hay, primero) y
	// los siguientes `limite` programas de un canal, ordenados por inicio.
	// channelID es el ID del canal en la app (channels.id); la resolución a
	// (provider_id, tvg_id) es interna.
	ProximosDeCanal(ctx context.Context, channelID string, ahora int64, limite int) ([]domain.Programa, error)
}
