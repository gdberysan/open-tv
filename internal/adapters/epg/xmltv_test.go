package epg

import (
	"os"
	"strings"
	"testing"
	"time"
)

// utcUnix es un helper de test para construir el epoch esperado a partir de
// una fecha/hora UTC legible, evitando "números mágicos" en las aserciones.
func utcUnix(t *testing.T, layout, value string) int64 {
	t.Helper()
	parsed, err := time.Parse(layout, value)
	if err != nil {
		t.Fatalf("utcUnix: fecha de test inválida %q: %v", value, err)
	}
	return parsed.UTC().Unix()
}

func openFixture(t *testing.T, name string) *os.File {
	t.Helper()
	f, err := os.Open("testdata/" + name) //nolint:gosec // name es un literal de fixture fijado por el propio test, no entrada externa
	if err != nil {
		t.Fatalf("no se pudo abrir fixture %q: %v", name, err)
	}
	t.Cleanup(func() { _ = f.Close() })
	return f
}

func TestParsearXMLTV_Basico(t *testing.T) {
	f := openFixture(t, "basico.xml")

	programas, err := ParsearXMLTV(f, 1<<20)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}

	if len(programas) != 2 {
		t.Fatalf("esperaba 2 programas, obtuve %d: %+v", len(programas), programas)
	}

	p0 := programas[0]
	if p0.ChannelID != "cnn.us" {
		t.Errorf("ChannelID = %q, quería %q", p0.ChannelID, "cnn.us")
	}
	if p0.Titulo != "The Lead" {
		t.Errorf("Titulo = %q, quería %q", p0.Titulo, "The Lead")
	}
	if p0.Subtitulo != "Breaking News" {
		t.Errorf("Subtitulo = %q, quería %q", p0.Subtitulo, "Breaking News")
	}
	if p0.Descripcion != "Cobertura de las noticias del día." {
		t.Errorf("Descripcion = %q, no coincide", p0.Descripcion)
	}

	wantInicio := utcUnix(t, "20060102150405 -0700", "20260826200000 +0000")
	wantFin := utcUnix(t, "20060102150405 -0700", "20260826210000 +0000")
	if p0.InicioUTC != wantInicio {
		t.Errorf("InicioUTC = %d, quería %d", p0.InicioUTC, wantInicio)
	}
	if p0.FinUTC != wantFin {
		t.Errorf("FinUTC = %d, quería %d", p0.FinUTC, wantFin)
	}

	p1 := programas[1]
	if p1.Titulo != "Anderson Cooper 360" {
		t.Errorf("Titulo = %q, quería %q", p1.Titulo, "Anderson Cooper 360")
	}
	// sub-title y desc son opcionales: ausentes en esta segunda entrada.
	if p1.Subtitulo != "" || p1.Descripcion != "" {
		t.Errorf("esperaba Subtitulo/Descripcion vacíos, obtuve %q/%q", p1.Subtitulo, p1.Descripcion)
	}
}

func TestParsearXMLTV_OffsetDistintoDeUTC(t *testing.T) {
	f := openFixture(t, "offset_no_utc.xml")

	programas, err := ParsearXMLTV(f, 1<<20)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if len(programas) != 1 {
		t.Fatalf("esperaba 1 programa, obtuve %d", len(programas))
	}

	// 20260826220000 +0200 == 20260826200000 +0000 en UTC.
	wantInicio := utcUnix(t, "20060102150405 -0700", "20260826200000 +0000")
	if programas[0].InicioUTC != wantInicio {
		t.Errorf("InicioUTC = %d, quería %d (conversión de +0200 a UTC incorrecta)", programas[0].InicioUTC, wantInicio)
	}
}

func TestParsearXMLTV_EntradasMalformadasSeDescartanElRestoSigue(t *testing.T) {
	f := openFixture(t, "entradas_malformadas.xml")

	programas, err := ParsearXMLTV(f, 1<<20)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}

	if len(programas) != 2 {
		t.Fatalf("esperaba 2 programas válidos (se descartan 2), obtuve %d: %+v", len(programas), programas)
	}
	if programas[0].Titulo != "Buena Entrada Uno" {
		t.Errorf("primer programa = %q, quería %q", programas[0].Titulo, "Buena Entrada Uno")
	}
	if programas[1].Titulo != "Buena Entrada Dos" {
		t.Errorf("segundo programa = %q, quería %q", programas[1].Titulo, "Buena Entrada Dos")
	}
}

func TestParsearXMLTV_VacioSinProgrammes(t *testing.T) {
	f := openFixture(t, "vacio.xml")

	programas, err := ParsearXMLTV(f, 1<<20)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if len(programas) != 0 {
		t.Fatalf("esperaba slice vacío, obtuve %d elementos", len(programas))
	}
}

func TestParsearXMLTV_CapDeTamano(t *testing.T) {
	const xmlDoc = `<?xml version="1.0" encoding="UTF-8"?>
<tv>
  <programme start="20260826200000 +0000" stop="20260826210000 +0000" channel="cnn.us">
    <title>The Lead</title>
  </programme>
</tv>
`
	r := strings.NewReader(xmlDoc)

	// maxBytes deliberadamente menor que el tamaño real del documento.
	_, err := ParsearXMLTV(r, 10)
	if err == nil {
		t.Fatal("esperaba error de tope de tamaño, obtuve nil")
	}
}

func TestParsearXMLTV_JustoEnElLimiteNoFalla(t *testing.T) {
	const xmlDoc = `<tv></tv>`
	r := strings.NewReader(xmlDoc)

	_, err := ParsearXMLTV(r, int64(len(xmlDoc)))
	if err != nil {
		t.Fatalf("un documento de exactamente maxBytes no debería fallar: %v", err)
	}
}

func TestParsearXMLTV_SinChannelSeDescarta(t *testing.T) {
	const xmlDoc = `<tv>
  <programme start="20260826200000 +0000" stop="20260826210000 +0000">
    <title>Sin canal</title>
  </programme>
</tv>`
	programas, err := ParsearXMLTV(strings.NewReader(xmlDoc), 1<<20)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if len(programas) != 0 {
		t.Fatalf("esperaba que se descartara la entrada sin channel, obtuve %d", len(programas))
	}
}

func TestParsearXMLTV_SinTituloSeDescarta(t *testing.T) {
	const xmlDoc = `<tv>
  <programme start="20260826200000 +0000" stop="20260826210000 +0000" channel="cnn.us">
  </programme>
</tv>`
	programas, err := ParsearXMLTV(strings.NewReader(xmlDoc), 1<<20)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if len(programas) != 0 {
		t.Fatalf("esperaba que se descartara la entrada sin title, obtuve %d", len(programas))
	}
}
