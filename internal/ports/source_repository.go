package ports

import (
	"context"
	"errors"
)

// ErrFuenteDuplicada señala que ya existe una fuente con esa URL (mismo id
// fnv64a). El handler que consuma SourceRepository.Add la traduce a un 409:
// añadir la misma URL dos veces no debe crear un provider fantasma silencioso.
var ErrFuenteDuplicada = errors.New("ya existe una fuente con esa URL")

// Source es una fuente IPTV configurable por el usuario ("bring your own"):
// una URL M3U remota o un fichero local subido. Vive sobre la misma tabla
// providers que ya usaba el proveedor IPTV-org fijo; type en esa tabla sigue
// constante a 'opensource' para toda fuente M3U — Kind distingue CÓMO se
// obtiene el M3U (url remota vs fichero subido), no el formato del catálogo.
type Source struct {
	ID    string
	Label string
	URL   string
	// Kind es "url" (M3U remoto) o "file" (fichero subido al datadir).
	Kind     string
	IsActive bool
	// UltimoSync es el Unix epoch del último TouchSync exitoso, 0 si la fuente
	// nunca se sincronizó.
	UltimoSync int64
	// Canales es el recuento de canales que tiene esta fuente en el catálogo
	// ahora mismo. Se calcula en List, no se persiste.
	Canales int
}

// SourceRepository gestiona las fuentes IPTV que el usuario ha dado de alta.
// Reemplaza el seed fijo de IPTV-org: una instalación limpia arranca con cero
// fuentes y el catálogo depende por completo de lo que el usuario añada aquí.
type SourceRepository interface {
	// List devuelve todas las fuentes, con Canales ya calculado.
	List(ctx context.Context) ([]Source, error)
	// Add da de alta una fuente nueva. El ID lo deriva el repositorio a partir
	// de la URL (determinista: la misma URL siempre produce el mismo ID), así
	// que el campo ID de s se ignora al insertar. Con una URL ya registrada
	// devuelve un error que envuelve ErrFuenteDuplicada.
	Add(ctx context.Context, s Source) error
	// Remove borra la fuente y, en cascada, sus canales y streams. No debe
	// afectar a ninguna otra fuente.
	Remove(ctx context.Context, id string) error
	// TouchSync marca cuándo fue el último sync exitoso de la fuente.
	TouchSync(ctx context.Context, id string, cuando int64) error
}
