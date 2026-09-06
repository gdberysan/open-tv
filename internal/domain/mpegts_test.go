package domain_test

import (
	"testing"

	"github.com/gdberysan/open-tv/internal/domain"
)

// Constructores de paquetes TS sintéticos. No se copian bytes de orígenes de
// terceros: la PAT y la PMT se fabrican aquí con las longitudes correctas.

// paqueteTS envuelve una sección PSI en un paquete de 188 bytes con PUSI y
// pointer field 0. Relleno 0xff, como hace cualquier multiplexor.
func paqueteTS(pid int, seccion []byte) []byte {
	p := make([]byte, 188)
	for i := range p {
		p[i] = 0xff
	}
	p[0] = 0x47
	//nolint:gosec
	p[1] = 0x40 | byte(pid>>8) // payload_unit_start_indicator
	//nolint:gosec
	p[2] = byte(pid)
	p[3] = 0x10 // solo carga útil, continuity counter 0
	p[4] = 0    // pointer field
	copy(p[5:], seccion)
	return p
}

// paqueteTSConAdaptacion es igual pero con campo de adaptación de `n` bytes
// delante del pointer field (adaptation_field_control = 3).
func paqueteTSConAdaptacion(pid int, n int, seccion []byte) []byte {
	p := paqueteTS(pid, nil)
	p[3] = 0x30
	//nolint:gosec
	p[4] = byte(n)
	for i := 5; i < 5+n; i++ {
		p[i] = 0x00
	}
	p[5+n] = 0 // pointer field
	copy(p[6+n:], seccion)
	return p
}

// seccionPSI arma table_id + section_length + cuerpo + CRC ficticio.
func seccionPSI(tableID byte, cuerpo []byte) []byte {
	largo := len(cuerpo) + 4 // + CRC32
	//nolint:gosec
	s := []byte{tableID, 0xb0 | byte(largo>>8), byte(largo)}
	s = append(s, cuerpo...)
	return append(s, 0, 0, 0, 0)
}

func pat(pmtPID int) []byte {
	//nolint:gosec
	cuerpo := []byte{
		0x00, 0x01, // transport_stream_id
		0xc1,       // version 0, current_next 1
		0x00, 0x00, // section_number, last_section_number
		0x00, 0x01, // program_number 1
		0xe0 | byte(pmtPID>>8), byte(pmtPID),
	}
	return seccionPSI(0x00, cuerpo)
}

func pmt(streams ...domain.StreamTS) []byte {
	cuerpo := []byte{
		0x00, 0x01, // program_number 1
		0xc1,       // version 0, current_next 1
		0x00, 0x00, // section_number, last_section_number
		0xe1, 0x00, // PCR_PID 0x100
		0xf0, 0x00, // program_info_length 0
	}
	for _, s := range streams {
		//nolint:gosec
		cuerpo = append(cuerpo, s.Tipo, 0xe0|byte(s.PID>>8), byte(s.PID), 0xf0, 0x00)
	}
	return seccionPSI(0x02, cuerpo)
}

func segmento(paquetes ...[]byte) []byte {
	var out []byte
	for _, p := range paquetes {
		out = append(out, p...)
	}
	return out
}

func TestParsearPMT_MPEG2ConMP2(t *testing.T) {
	prefijo := segmento(
		paqueteTS(0, pat(0x1000)),
		paqueteTS(0x1000, pmt(domain.StreamTS{Tipo: 0x02, PID: 0x100}, domain.StreamTS{Tipo: 0x03, PID: 0x101})),
	)
	got, err := domain.ParsearPMT(prefijo)
	if err != nil {
		t.Fatalf("ParsearPMT: %v", err)
	}
	if len(got) != 2 || got[0].Tipo != 0x02 || got[0].PID != 0x100 || got[1].Tipo != 0x03 || got[1].PID != 0x101 {
		t.Errorf("streams = %+v", got)
	}
}

// La PAT y la PMT no tienen por qué ir en los dos primeros paquetes: puede
// haber paquetes de datos (sin PUSI) delante, y la PMT puede llevar campo de
// adaptación. Ambas cosas se ven en orígenes reales.
func TestParsearPMT_SaltaPaquetesDeDatosYCampoDeAdaptacion(t *testing.T) {
	datos := paqueteTS(0x100, nil)
	datos[1] = 0x01 // sin PUSI
	prefijo := segmento(
		datos,
		paqueteTS(0, pat(0x0fff)),
		datos,
		paqueteTSConAdaptacion(0x0fff, 7, pmt(domain.StreamTS{Tipo: 0x1b, PID: 0x0d3})),
	)
	got, err := domain.ParsearPMT(prefijo)
	if err != nil {
		t.Fatalf("ParsearPMT: %v", err)
	}
	if len(got) != 1 || got[0].Tipo != 0x1b {
		t.Errorf("streams = %+v, quiero solo H.264", got)
	}
}

func TestParsearPMT_PMTFueraDelPrefijoEsError(t *testing.T) {
	prefijo := segmento(paqueteTS(0, pat(0x1000)))
	if _, err := domain.ParsearPMT(prefijo); err == nil {
		t.Error("sin PMT en el prefijo tiene que fallar, no inventarse streams")
	}
}

func TestParsearPMT_SyncRotoEsError(t *testing.T) {
	prefijo := segmento(paqueteTS(0, pat(0x1000)))
	prefijo[0] = 0x00
	if _, err := domain.ParsearPMT(prefijo); err == nil {
		t.Error("un prefijo sin byte de sincronía no es MPEG-TS")
	}
}

func TestParsearPMT_PrefijoVacioEsError(t *testing.T) {
	if _, err := domain.ParsearPMT(nil); err == nil {
		t.Error("prefijo vacío tiene que fallar")
	}
}

// Descriptores en el bucle de ES (ES_info_length > 0): hay que saltarlos,
// no leerlos como si fueran otro stream.
func TestParsearPMT_SaltaDescriptoresDeES(t *testing.T) {
	cuerpo := []byte{
		0x00, 0x01, 0xc1, 0x00, 0x00, 0xe1, 0x00, 0xf0, 0x00,
		0x1b, 0xe1, 0x00, 0xf0, 0x03, 0x0a, 0x01, 0x00, // H.264 con 3 bytes de descriptor
		0x0f, 0xe1, 0x01, 0xf0, 0x00, // AAC
	}
	prefijo := segmento(paqueteTS(0, pat(0x1000)), paqueteTS(0x1000, seccionPSI(0x02, cuerpo)))
	got, err := domain.ParsearPMT(prefijo)
	if err != nil {
		t.Fatalf("ParsearPMT: %v", err)
	}
	if len(got) != 2 || got[0].Tipo != 0x1b || got[1].Tipo != 0x0f {
		t.Errorf("streams = %+v", got)
	}
}
