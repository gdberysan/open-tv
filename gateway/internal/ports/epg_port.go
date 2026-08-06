package ports

import (
	"context"
	"io"

	"github.com/tu-org/iptv-ecosystem/gateway/internal/domain"
)

type EPGPort interface {
	// ParseStream procesa un reader XMLTV en modo streaming.
	// Llama a onEntry por cada entrada parseada; nunca carga el XML completo.
	ParseStream(ctx context.Context, r io.Reader, onEntry func(domain.EPGEntry) error) error
}
