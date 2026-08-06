package epg

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/tu-org/iptv-ecosystem/gateway/internal/domain"
)

// Parser implementa ports.EPGPort para decodificar archivos XMLTV.
type Parser struct{}

func NewParser() *Parser {
	return &Parser{}
}

// xmltvProgramme es una estructura temporal pequeña para parsear un único elemento <programme>
type xmltvProgramme struct {
	ChannelID string `xml:"channel,attr"`
	Start     string `xml:"start,attr"`
	Stop      string `xml:"stop,attr"`
	Title     string `xml:"title"`
	Desc      string `xml:"desc"`
}

// ParseStream lee el XML token a token, evitando cargarlo completo en memoria.
func (p *Parser) ParseStream(ctx context.Context, r io.Reader, onEntry func(domain.EPGEntry) error) error {
	decoder := xml.NewDecoder(r)

	for {
		// Chequear cancelación del contexto
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		t, err := decoder.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return fmt.Errorf("error leyendo token XML: %w", err)
		}

		// Buscamos el elemento inicial <programme>
		switch se := t.(type) {
		case xml.StartElement:
			if se.Name.Local == "programme" {
				var prog xmltvProgramme
				// Usamos DecodeElement que solo lee este sub-árbol en memoria
				if err := decoder.DecodeElement(&prog, &se); err != nil {
					return fmt.Errorf("error decodificando programme: %w", err)
				}

				entry, err := parseProgramme(prog)
				if err == nil {
					// Disparamos el callback
					if cbErr := onEntry(entry); cbErr != nil {
						return fmt.Errorf("error en callback onEntry: %w", cbErr)
					}
				}
				// Si hay error de parseo de fecha, lo ignoramos y pasamos al siguiente
			}
		}
	}

	return nil
}

// parseProgramme convierte el formato XMLTV a la entidad pura EPGEntry.
func parseProgramme(prog xmltvProgramme) (domain.EPGEntry, error) {
	// Formato típico XMLTV: "20240520140000 +0200" o "20240520140000"
	layout := "20060102150405 -0700"
	layoutNoTZ := "20060102150405"

	parseTime := func(value string) (time.Time, error) {
		v := strings.TrimSpace(value)
		if len(v) >= 14 {
			if strings.Contains(v, "+") || strings.Contains(v, "-") && len(v) > 14 {
				return time.Parse(layout, v)
			}
			// Fallback si no hay zona horaria explícita
			return time.Parse(layoutNoTZ, v[:14])
		}
		return time.Time{}, fmt.Errorf("formato de tiempo inválido: %s", v)
	}

	start, err := parseTime(prog.Start)
	if err != nil {
		return domain.EPGEntry{}, err
	}

	stop, err := parseTime(prog.Stop)
	if err != nil {
		return domain.EPGEntry{}, err
	}

	return domain.EPGEntry{
		ChannelID:   domain.ChannelID(prog.ChannelID),
		Title:       strings.TrimSpace(prog.Title),
		Description: strings.TrimSpace(prog.Desc),
		StartAt:     start,
		EndAt:       stop,
	}, nil
}
