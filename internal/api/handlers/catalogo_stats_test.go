package handlers

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/gdberysan/open-tv/internal/adapters/db"
	"github.com/gdberysan/open-tv/internal/domain"
	"github.com/gdberysan/open-tv/internal/ports"
)

// El agregado del catálogo cuenta los mirrors cuyo vídeo ningún navegador
// decodifica (codec_ok = 0), aparte de web_ok/web_no.
func TestDBCatalogoStatsCuentaCodecNo(t *testing.T) {
	sqlDB, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer func() { _ = sqlDB.Close() }()
	if _, err := sqlDB.Exec(`
		INSERT INTO providers (id, type, base_url, priority, is_active, created_at, updated_at)
		VALUES ('opensource', 'opensource', 'http://test.invalid/index.m3u', 100, 1, 0, 0)
	`); err != nil {
		t.Fatalf("seed provider: %v", err)
	}
	ctx := context.Background()
	chRepo := db.NewChannelRepository(sqlDB)
	if err := chRepo.Save(ctx, domain.Channel{ID: "c1", Name: "AMC (720p)", ProviderID: "opensource", ProviderType: domain.ProviderOpenSource}); err != nil {
		t.Fatalf("Save canal: %v", err)
	}
	stRepo := db.NewStreamRepository(sqlDB)
	for _, id := range []string{"s-mpeg2", "s-h264", "s-sin-sondear"} {
		if err := stRepo.Save(ctx, domain.Stream{ID: id, ChannelID: "c1", URL: "http://o/" + id + ".m3u8", Protocol: domain.ProtocolHLS}); err != nil {
			t.Fatalf("Save %s: %v", id, err)
		}
	}
	if err := stRepo.MarkBatch(ctx, []ports.StreamHealth{
		{StreamID: "s-mpeg2", IsAlive: true, Codec: domain.CodecNo, Codecs: "mpeg2video,mp2", CodecSondeado: true},
		{StreamID: "s-h264", IsAlive: true, Codec: domain.CodecOK, Codecs: "h264,aac", CodecSondeado: true},
		{StreamID: "s-sin-sondear", IsAlive: true},
	}); err != nil {
		t.Fatalf("MarkBatch: %v", err)
	}

	resumen, err := NewDBCatalogoStats(sqlDB).Resumen(ctx)
	if err != nil {
		t.Fatalf("Resumen: %v", err)
	}
	if resumen["codec_no"] != 1 {
		t.Errorf("codec_no = %v, quiero 1", resumen["codec_no"])
	}
	if resumen["streams_totales"] != 3 {
		t.Errorf("streams_totales = %v, quiero 3", resumen["streams_totales"])
	}
}

// imagen_p50_ms es la mediana de los imagen_ms > 0 (tiempo-hasta-la-imagen
// §3.5); con n par se toma el elemento n/2 de la lista ORDENADA (índice 1 de
// [1000, 3000] con n=2 → 3000; documentado así a propósito, no la media).
// sin_imagen cuenta los mirrors que domain.SinImagen ya salta AHORA: 2 fallos
// reales dentro de la ventana.
func TestDBCatalogoStatsImagenP50YSinImagen(t *testing.T) {
	sqlDB, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer func() { _ = sqlDB.Close() }()
	if _, err := sqlDB.Exec(`
		INSERT INTO providers (id, type, base_url, priority, is_active, created_at, updated_at)
		VALUES ('opensource', 'opensource', 'http://test.invalid/index.m3u', 100, 1, 0, 0)
	`); err != nil {
		t.Fatalf("seed provider: %v", err)
	}
	ctx := context.Background()
	chRepo := db.NewChannelRepository(sqlDB)
	if err := chRepo.Save(ctx, domain.Channel{ID: "c1", Name: "Canal (720p)", ProviderID: "opensource", ProviderType: domain.ProviderOpenSource}); err != nil {
		t.Fatalf("Save canal: %v", err)
	}
	stRepo := db.NewStreamRepository(sqlDB)
	for _, id := range []string{"s-1000", "s-3000", "s-caido"} {
		if err := stRepo.Save(ctx, domain.Stream{ID: id, ChannelID: "c1", URL: "http://o/" + id + ".m3u8", Protocol: domain.ProtocolHLS}); err != nil {
			t.Fatalf("Save %s: %v", id, err)
		}
	}
	// Dos éxitos con imagen_ms distintos.
	if err := stRepo.RegistrarDesenlace(ctx, "http://o/s-1000.m3u8", ports.DesenlaceMirror{Resultado: "iniciado", MsPrimerFrame: 1000}); err != nil {
		t.Fatalf("RegistrarDesenlace s-1000: %v", err)
	}
	if err := stRepo.RegistrarDesenlace(ctx, "http://o/s-3000.m3u8", ports.DesenlaceMirror{Resultado: "iniciado", MsPrimerFrame: 3000}); err != nil {
		t.Fatalf("RegistrarDesenlace s-3000: %v", err)
	}
	// Dos fallos reales consecutivos: cruza domain.UmbralFallosReales.
	for i := 0; i < 2; i++ {
		if err := stRepo.RegistrarDesenlace(ctx, "http://o/s-caido.m3u8", ports.DesenlaceMirror{Resultado: "fallo", Motivo: "caido"}); err != nil {
			t.Fatalf("RegistrarDesenlace s-caido %d: %v", i, err)
		}
	}

	resumen, err := NewDBCatalogoStats(sqlDB).Resumen(ctx)
	if err != nil {
		t.Fatalf("Resumen: %v", err)
	}
	if resumen["imagen_p50_ms"] != int64(3000) {
		t.Errorf("imagen_p50_ms = %v, quiero 3000 (elemento n/2 de [1000,3000])", resumen["imagen_p50_ms"])
	}
	if resumen["sin_imagen"] != 1 {
		t.Errorf("sin_imagen = %v, quiero 1", resumen["sin_imagen"])
	}
}

// Sin ningún imagen_ms > 0 en toda la tabla, la mediana es 0, no NaN ni un
// pánico de índice fuera de rango.
func TestDBCatalogoStatsImagenP50VacioEsCero(t *testing.T) {
	sqlDB, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer func() { _ = sqlDB.Close() }()

	resumen, err := NewDBCatalogoStats(sqlDB).Resumen(context.Background())
	if err != nil {
		t.Fatalf("Resumen: %v", err)
	}
	if resumen["imagen_p50_ms"] != int64(0) {
		t.Errorf("imagen_p50_ms = %v, quiero 0", resumen["imagen_p50_ms"])
	}
	if resumen["sin_imagen"] != 0 {
		t.Errorf("sin_imagen = %v, quiero 0", resumen["sin_imagen"])
	}
}
