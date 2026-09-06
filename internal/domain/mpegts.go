package domain

import (
	"errors"
	"fmt"
)

// StreamTS es una entrada del bucle de streams elementales de una PMT.
type StreamTS struct {
	Tipo byte
	PID  uint16
}

const (
	tamanoPaqueteTS = 188
	syncTS          = 0x47
	pidPAT          = 0
	tablaPAT        = 0x00
	tablaPMT        = 0x02
)

// ErrSinPMT: el prefijo era MPEG-TS válido pero la PAT o la PMT no cabían
// en él. Es un "no se sabe", no un "no sirve".
var ErrSinPMT = errors.New("mpegts: PAT/PMT no encontradas en el prefijo")

// ParsearPMT lee un PREFIJO de un segmento MPEG-TS (no hace falta el
// segmento entero: medido en orígenes reales, PAT y PMT van en los paquetes
// 2 y 3) y devuelve los streams elementales del primer programa. Cualquier
// cosa que no cuadre es error: quien llama lo traduce a CodecUnknown, nunca
// a CodecNo.
func ParsearPMT(prefijo []byte) ([]StreamTS, error) {
	n := len(prefijo) / tamanoPaqueteTS
	if n == 0 {
		return nil, ErrSinPMT
	}
	pmtPID := -1
	for i := 0; i < n; i++ {
		p := prefijo[i*tamanoPaqueteTS : (i+1)*tamanoPaqueteTS]
		if p[0] != syncTS {
			return nil, fmt.Errorf("mpegts: paquete %d sin byte de sincronía", i)
		}
		pusi := p[1]&0x40 != 0
		pid := int(p[1]&0x1f)<<8 | int(p[2])
		if !pusi {
			continue
		}
		seccion, ok := seccionDelPaquete(p)
		if !ok {
			continue
		}
		switch {
		case pid == pidPAT && pmtPID < 0:
			pmtPID = pidPMTDesdePAT(seccion)
		case pmtPID >= 0 && pid == pmtPID:
			return streamsDesdePMT(seccion)
		}
	}
	return nil, ErrSinPMT
}

// seccionDelPaquete salta la cabecera, el campo de adaptación si lo hay y el
// pointer field, y devuelve el principio de la sección PSI.
func seccionDelPaquete(p []byte) ([]byte, bool) {
	afc := (p[3] >> 4) & 0x3
	if afc == 0 || afc == 2 { // reservado / solo adaptación: sin carga útil
		return nil, false
	}
	off := 4
	if afc == 3 {
		off += 1 + int(p[4])
	}
	if off >= len(p) {
		return nil, false
	}
	off += 1 + int(p[off]) // pointer field
	if off >= len(p) {
		return nil, false
	}
	return p[off:], true
}

// pidPMTDesdePAT devuelve el PID de la PMT del primer programa distinto de 0
// (el 0 es la NIT). -1 si la sección no es una PAT o no tiene programas.
func pidPMTDesdePAT(sec []byte) int {
	if len(sec) < 12 || sec[0] != tablaPAT {
		return -1
	}
	largo := int(sec[1]&0x0f)<<8 | int(sec[2])
	fin := 3 + largo - 4 // sin el CRC32
	if fin > len(sec) {
		fin = len(sec)
	}
	for i := 8; i+4 <= fin; i += 4 {
		programa := int(sec[i])<<8 | int(sec[i+1])
		pid := int(sec[i+2]&0x1f)<<8 | int(sec[i+3])
		if programa != 0 {
			return pid
		}
	}
	return -1
}

func streamsDesdePMT(sec []byte) ([]StreamTS, error) {
	if len(sec) < 12 || sec[0] != tablaPMT {
		return nil, errors.New("mpegts: la sección en el PID de la PMT no es una PMT")
	}
	largo := int(sec[1]&0x0f)<<8 | int(sec[2])
	fin := 3 + largo - 4
	if fin > len(sec) {
		return nil, errors.New("mpegts: PMT truncada en el prefijo")
	}
	infoPrograma := int(sec[10]&0x0f)<<8 | int(sec[11])
	var out []StreamTS
	for i := 12 + infoPrograma; i+5 <= fin; {
		tipo := sec[i]
		pid := uint16(sec[i+1]&0x1f)<<8 | uint16(sec[i+2])
		infoES := int(sec[i+3]&0x0f)<<8 | int(sec[i+4])
		out = append(out, StreamTS{Tipo: tipo, PID: pid})
		i += 5 + infoES
	}
	if len(out) == 0 {
		return nil, errors.New("mpegts: PMT sin streams elementales")
	}
	return out, nil
}
