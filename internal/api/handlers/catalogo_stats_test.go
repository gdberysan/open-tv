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
