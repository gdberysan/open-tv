package api_test

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/gdberysan/open-tv/gateway/internal/adapters/db"
	"github.com/gdberysan/open-tv/gateway/internal/api"
	"github.com/gdberysan/open-tv/gateway/internal/domain"
	"github.com/gdberysan/open-tv/gateway/internal/ports"
)

type repoVacio struct{}

func (repoVacio) Save(context.Context, domain.Channel) error        { return nil }
func (repoVacio) SaveBatch(context.Context, []domain.Channel) error { return nil }
func (repoVacio) FindByID(context.Context, domain.ChannelID) (domain.Channel, error) {
	return domain.Channel{}, nil
}
func (repoVacio) FindFiltered(context.Context, ports.ChannelFilter) ([]domain.Channel, error) {
	return nil, nil
}
func (repoVacio) Search(context.Context, string, int) ([]domain.Channel, error) { return nil, nil }
func (repoVacio) Delete(context.Context, domain.ChannelID) error                { return nil }
func (repoVacio) DeleteStale(context.Context, string, time.Time) (int64, error) { return 0, nil }
func (repoVacio) CountFiltered(context.Context, ports.ChannelFilter) (int, error) {
	return 0, nil
}
func (repoVacio) Random(context.Context, ports.ChannelFilter) (domain.Channel, error) {
	return domain.Channel{}, errors.New("vacío")
}

func (repoVacio) Countries(context.Context) ([]ports.Faceta, error)  { return nil, nil }
func (repoVacio) Categories(context.Context) ([]ports.Faceta, error) { return nil, nil }

type provVacio struct{}

func (provVacio) ID() string                        { return "test" }
func (provVacio) Type() domain.ProviderType         { return domain.ProviderOpenSource }
func (provVacio) HealthCheck(context.Context) error { return nil }
func (provVacio) GetLiveChannels(context.Context) ([]domain.Channel, error) {
	return nil, nil
}

// Una caché de provider vacía da error, no cadena vacía: devolver ("", nil)
// haría que el handler sirviera una URL vacía como si fuese válida.
func (provVacio) GetStreamURL(context.Context, domain.ChannelID) (string, error) {
	return "", errors.New("caché vacía")
}

type streamsVacio struct{}

func (streamsVacio) Save(context.Context, domain.Stream) error        { return nil }
func (streamsVacio) SaveBatch(context.Context, []domain.Stream) error { return nil }
func (streamsVacio) FindAll(context.Context) ([]domain.Stream, error) { return nil, nil }
func (streamsVacio) FindByChannelID(context.Context, domain.ChannelID) ([]domain.Stream, error) {
	return nil, nil
}
func (streamsVacio) FindBestByChannelID(context.Context, domain.ChannelID) (domain.Stream, error) {
	return domain.Stream{}, sql.ErrNoRows
}
func (streamsVacio) MarkAlive(context.Context, string, int64) error        { return nil }
func (streamsVacio) MarkDead(context.Context, string) error                { return nil }
func (streamsVacio) MarkBatch(context.Context, []ports.StreamHealth) error { return nil }

type syncVacio struct{}

func (syncVacio) LastSuccess() time.Time { return time.Time{} }

// Este test usa el router REAL, no una tabla de rutas duplicada en el test: si
// el helper de los tests de handlers construye su propia tabla, puede derivar
// de la de producción sin que nadie se entere.
func TestTablaDeRutas(t *testing.T) {
	sqlDB, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer sqlDB.Close()

	r := api.NewRouter(slog.New(slog.DiscardHandler), repoVacio{}, provVacio{},
		streamsVacio{}, sqlDB, syncVacio{})

	casos := []struct {
		ruta   string
		quiero int
	}{
		{"/health", http.StatusOK},
		{"/channels", http.StatusOK},
		// Rutas literales que conviven con /{id}/health: si chi las absorbiera
		// como parámetro, los selectores recibirían basura en vez de facetas.
		{"/channels/countries", http.StatusOK},
		{"/channels/categories", http.StatusOK},
		{"/channels/stream?id=x", http.StatusNotFound},
		{"/channels/loquesea/health", http.StatusNotFound},
		// La guía se retiró.
		{"/epg/now", http.StatusNotFound},
	}
	for _, c := range casos {
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, c.ruta, nil))
		if rr.Code != c.quiero {
			t.Errorf("%s = %d, quiero %d", c.ruta, rr.Code, c.quiero)
		}
	}
}
