package epg

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/tu-org/iptv-ecosystem/gateway/internal/domain"
)

const mockXMLTV = `<?xml version="1.0" encoding="UTF-8"?>
<tv generator-info-name="mock">
  <channel id="123">
    <display-name>Mock TV</display-name>
  </channel>
  <programme start="20240520140000 +0200" stop="20240520150000 +0200" channel="123">
    <title lang="es">Noticias</title>
    <desc lang="es">Noticias del día</desc>
  </programme>
  <programme start="20240520150000" stop="20240520160000" channel="123">
    <title lang="es">Deportes</title>
    <desc lang="es">Resumen deportivo sin timezone</desc>
  </programme>
</tv>
`

func TestParser_ParseStream(t *testing.T) {
	parser := NewParser()
	r := strings.NewReader(mockXMLTV)

	var entries []domain.EPGEntry

	onEntry := func(entry domain.EPGEntry) error {
		entries = append(entries, entry)
		return nil
	}

	err := parser.ParseStream(context.Background(), r, onEntry)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("Expected 2 entries, got %d", len(entries))
	}

	// Verificar la primera entrada (con timezone)
	e1 := entries[0]
	if e1.ChannelID != "123" {
		t.Errorf("e1 ChannelID = %s, want 123", e1.ChannelID)
	}
	if e1.Title != "Noticias" {
		t.Errorf("e1 Title = %s, want Noticias", e1.Title)
	}

	// La fecha fue parseada usando el layout de Go
	// 20240520140000 +0200 -> 2024-05-20 14:00:00 +0200
	if e1.StartAt.Year() != 2024 || e1.StartAt.Month() != time.May || e1.StartAt.Hour() != 14 {
		t.Errorf("e1 StartAt = %v, want 2024-05-20 14:00:00", e1.StartAt)
	}

	// Verificar la segunda entrada (sin timezone explícita, hace fallback)
	e2 := entries[1]
	if e2.Title != "Deportes" {
		t.Errorf("e2 Title = %s, want Deportes", e2.Title)
	}
	if e2.StartAt.Hour() != 15 {
		t.Errorf("e2 StartAt.Hour = %d, want 15", e2.StartAt.Hour())
	}
}
