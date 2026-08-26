// Package epg contiene los adapters de la guía electrónica de programación
// (EPG): descarga (Fetcher) y parseo (ParsearXMLTV) del formato XMLTV. Es un
// adapter autónomo — no depende de ningún puerto ni de otras piezas de P2 —
// para que el Syncer (Tarea 5) lo componga con el repositorio de EPG.
package epg

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/gdberysan/open-tv/internal/domain"
)

// xmltvTimeLayout es el formato de fecha/hora que usa XMLTV para start/stop:
// "AñoMesDíaHoraMinSeg Offset", p. ej. "20260826200000 +0000".
const xmltvTimeLayout = "20060102150405 -0700"

// xmltvTimeLayoutSinOffset cubre las guías que omiten el offset (algunas
// fuentes lo hacen); sin componente de zona, time.Parse asume UTC.
const xmltvTimeLayoutSinOffset = "20060102150405"

// xmlProgramme es la representación cruda de un <programme> tal y como lo
// entrega encoding/xml, antes de validar y convertir a domain.Programa.
type xmlProgramme struct {
	XMLName  xml.Name `xml:"programme"`
	Start    string   `xml:"start,attr"`
	Stop     string   `xml:"stop,attr"`
	Channel  string   `xml:"channel,attr"`
	Title    string   `xml:"title"`
	SubTitle string   `xml:"sub-title"`
	Desc     string   `xml:"desc"`
}

// ParsearXMLTV parsea una guía XMLTV en streaming (sin cargar el documento
// completo en memoria) y devuelve los domain.Programa válidos que contiene.
//
// Streaming: se usa xml.Decoder.Token() para recorrer el documento token a
// token; solo al encontrar el StartElement de un <programme> se decodifica
// ese subárbol puntual con DecodeElement (que consume hasta su
// EndElement correspondiente y vuelve al bucle de Token()). Así el coste de
// memoria es O(un <programme>), no O(documento completo) — una guía XMLTV
// real puede tener cientos de miles de entradas.
//
// maxBytes aplica un tope de tamaño sobre r con el mismo patrón "N+1" que
// parseM3UStream (ver internal/adapters/providers/opensource/provider.go):
// io.LimitedReader con N = maxBytes+1 deja pasar un documento de exactamente
// maxBytes bytes (N termina en 1, no en 0) pero any exceso hace que N llegue
// a 0, lo que se traduce en un error de tope claro en vez de un error de
// parseo XML confuso (truncamiento a mitad de un tag).
//
// Tolerancia: una entrada <programme> con fecha ilegible, o a la que le
// falte start/stop/channel/title, se descarta silenciosamente — no aborta
// el resto del parseo. Solo un documento realmente malformado (XML inválido
// que rompe el propio decoder) o el tope de tamaño excedido devuelven error.
func ParsearXMLTV(r io.Reader, maxBytes int64) ([]domain.Programa, error) {
	limited := &io.LimitedReader{R: r, N: maxBytes + 1}
	dec := xml.NewDecoder(limited)

	var programas []domain.Programa

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			if limited.N <= 0 {
				return nil, fmt.Errorf("epg.ParsearXMLTV: documento excede el tamaño máximo de %d bytes", maxBytes)
			}
			return nil, fmt.Errorf("epg.ParsearXMLTV: fallo al parsear XML: %w", err)
		}

		se, ok := tok.(xml.StartElement)
		if !ok || se.Name.Local != "programme" {
			continue
		}

		var raw xmlProgramme
		if err := dec.DecodeElement(&raw, &se); err != nil {
			if limited.N <= 0 {
				return nil, fmt.Errorf("epg.ParsearXMLTV: documento excede el tamaño máximo de %d bytes", maxBytes)
			}
			return nil, fmt.Errorf("epg.ParsearXMLTV: fallo al decodificar <programme>: %w", err)
		}

		programa, ok := programaDesdeXML(raw)
		if !ok {
			continue // entrada inválida: se descarta, el parseo sigue
		}
		programas = append(programas, programa)
	}

	if limited.N <= 0 {
		return nil, fmt.Errorf("epg.ParsearXMLTV: documento excede el tamaño máximo de %d bytes", maxBytes)
	}

	return programas, nil
}

// programaDesdeXML valida y convierte un xmlProgramme crudo a domain.Programa.
// channel, start, stop y title son obligatorios para que la entrada sea útil;
// sub-title y desc son opcionales. Devuelve ok=false si falta algún campo
// obligatorio o si start/stop no se pueden parsear como fecha XMLTV — nunca
// hace panic ni propaga el error, porque la entrada simplemente se descarta.
func programaDesdeXML(raw xmlProgramme) (domain.Programa, bool) {
	if raw.Channel == "" || raw.Start == "" || raw.Stop == "" || raw.Title == "" {
		return domain.Programa{}, false
	}

	inicio, err := parseXMLTVTime(raw.Start)
	if err != nil {
		return domain.Programa{}, false
	}
	fin, err := parseXMLTVTime(raw.Stop)
	if err != nil {
		return domain.Programa{}, false
	}

	return domain.Programa{
		ChannelID:   raw.Channel,
		InicioUTC:   inicio,
		FinUTC:      fin,
		Titulo:      raw.Title,
		Subtitulo:   raw.SubTitle,
		Descripcion: raw.Desc,
	}, true
}

// parseXMLTVTime convierte una fecha XMLTV ("20060102150405 -0700", y como
// fallback "20060102150405" para las fuentes que omiten el offset) a epoch
// UTC en segundos.
func parseXMLTVTime(s string) (int64, error) {
	s = strings.TrimSpace(s)

	if t, err := time.Parse(xmltvTimeLayout, s); err == nil {
		return t.UTC().Unix(), nil
	}
	if t, err := time.Parse(xmltvTimeLayoutSinOffset, s); err == nil {
		return t.UTC().Unix(), nil
	}
	return 0, fmt.Errorf("epg: fecha XMLTV ilegible: %q", s)
}
