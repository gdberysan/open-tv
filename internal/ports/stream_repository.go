package ports

import (
	"context"
	"time"

	"github.com/gdberysan/open-tv/internal/domain"
)

// StreamHealth es el resultado de un chequeo de salud listo para persistir.
type StreamHealth struct {
	StreamID  string
	IsAlive   bool
	LatencyMs int64
	// Web es el veredicto de reproducibilidad en navegador. WebUnknown
	// (el cero valor) significa "no se pudo preguntar" y NO pisa lo que ya
	// hubiera guardado.
	Web domain.WebSupport
	// Codec es el veredicto de la sonda del primer segmento (spec
	// salud-por-segmento). CodecSondeado dice si la sonda CORRIÓ en este
	// chequeo: si corrió, se sella codec_checked_at aunque el veredicto sea
	// CodecUnknown; un Unknown nunca pisa un veredicto anterior.
	Codec         domain.CodecSupport
	Codecs        string
	CodecSondeado bool
	// Audio sale de la misma PMT que Codec (spec tiempo-hasta-la-imagen
	// §3.1). Unknown nunca pisa lo que ya hubiera guardado.
	Audio domain.AudioSupport
}

// MirrorHealth es un stream de un canal con su salud, tal y como lo consume el
// failover del cliente. WebOK es tri-estado (WebUnknown = sin comprobar).
// airplay_ok NO se incluye: no se persiste (se sondea bajo demanda).
type MirrorHealth struct {
	URL       string
	IsAlive   bool
	LatencyMs int64
	WebOK     domain.WebSupport
	// Codec: CodecNo = ningún navegador decodifica su vídeo; el cliente lo
	// salta. Codecs es la cadena corta para el mensaje ("mpeg2video,mp2").
	Codec           domain.CodecSupport
	Codecs          string
	Audio           domain.AudioSupport
	ImagenMs        int64 // último tiempo real hasta la imagen; 0 = nunca
	FallosReales    int
	UltimoDesenlace time.Time // cero = nunca
	UltimoMotivo    string
}

// DesenlaceMirror es lo que el reproductor cuenta de un intento REAL sobre un
// mirror (spec tiempo-hasta-la-imagen §3.1). Resultado: "iniciado" | "fallo".
type DesenlaceMirror struct {
	Resultado     string
	Motivo        string
	MsPrimerFrame int64
}

// ImagenCanal es el resumen por canal para la tarjeta (GET /channels/imagen):
// solo existe para canales con algún desenlace registrado.
type ImagenCanal struct {
	ChannelID domain.ChannelID
	ImagenMs  int64 // menor imagen_ms > 0 entre sus mirrors vivos; 0 = ninguno
	SinImagen bool  // TODOS sus mirrors vivos están saltados (códec o §3.3)
}

// RegistradorDesenlaces es el puerto de ESCRITURA del bucle de verdad. Va
// aparte de StreamRepository porque el router recibe el repo de solo lectura;
// cmd/open-tv/main.go inyecta el del pool de escritura por api.Options.
type RegistradorDesenlaces interface {
	// RegistrarDesenlace aplica el desenlace a TODAS las filas con esa URL.
	// URL desconocida = no-op sin error (un mirror podado no rompe nada).
	RegistrarDesenlace(ctx context.Context, url string, d DesenlaceMirror) error
}

type StreamRepository interface {
	Save(ctx context.Context, s domain.Stream) error
	SaveBatch(ctx context.Context, streams []domain.Stream) error
	// FindAll devuelve el catálogo completo; lo consume el health-worker.
	FindAll(ctx context.Context) ([]domain.Stream, error)
	FindByChannelID(ctx context.Context, channelID domain.ChannelID) ([]domain.Stream, error)
	FindBestByChannelID(ctx context.Context, channelID domain.ChannelID) (domain.Stream, error) // menor latencia, is_alive=true
	// FindMirrorsByChannelID devuelve TODOS los mirrors del canal ordenados:
	// vivos primero; dentro de vivos, con audio antes que sin audio; luego
	// imagen conocida (imagen_ms > 0) ascendente antes que desconocida; luego
	// latencia ascendente; los muertos van al final.
	FindMirrorsByChannelID(ctx context.Context, channelID domain.ChannelID) ([]MirrorHealth, error)
	MarkAlive(ctx context.Context, streamID string, latencyMs int64) error
	MarkDead(ctx context.Context, streamID string) error
	// MarkBatch aplica todos los resultados en una sola transacción. El worker
	// chequea ~12k streams por pasada: hacerlo con un UPDATE suelto por stream
	// son ~12k transacciones implícitas y otros tantos fsync, y con
	// MaxOpenConns(1) ese tiempo es API congelada.
	MarkBatch(ctx context.Context, resultados []StreamHealth) error
	// DeleteStale borra los streams de la fuente cuyo last_seen_at sea anterior
	// a `before`. Mismo scoping por fuente que la poda de canales: la tabla
	// streams no tiene provider_id, así que se resuelve por su canal.
	DeleteStale(ctx context.Context, providerID string, before time.Time) (int64, error)
	// CabecerasPorURL devuelve las cabeceras que el origen exige para esa URL.
	// Una URL desconocida devuelve cadenas vacías y error nil: no es un fallo,
	// es "usa las de siempre".
	CabecerasPorURL(ctx context.Context, url string) (referrer, userAgent string, err error)
	// ImagenPorCanal resume el tiempo hasta la imagen por canal (spec
	// tiempo-hasta-la-imagen §3.4): solo canales con algún desenlace
	// registrado. ahora es la referencia de tiempo para domain.SinImagen.
	ImagenPorCanal(ctx context.Context, ahora time.Time) ([]ImagenCanal, error)
}

// VerificadorURLs responde si una URL es, exacta, la de un stream del
// catálogo. Lo usa el proxy HLS para autorizar las URLs de nivel superior,
// que el cliente construye con la URL cruda del mirror y no llevan firma. Va
// aparte de StreamRepository para no obligar a todos sus dobles de test a
// implementarlo.
type VerificadorURLs interface {
	ExisteURL(ctx context.Context, url string) (bool, error)
}
