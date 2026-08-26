package netx_test

import (
	"net"
	"strconv"
	"testing"

	"github.com/gdberysan/open-tv/internal/netx"
)

func TestEscuchaConFallbackSaltaAlSiguientePuerto(t *testing.T) {
	// Ocupamos un puerto real y pedimos ESE. El fallback debe darnos el +1.
	ocupado, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("preparando el puerto ocupado: %v", err)
	}
	defer func() { _ = ocupado.Close() }()

	_, puertoStr, _ := net.SplitHostPort(ocupado.Addr().String())
	puerto, _ := strconv.Atoi(puertoStr)

	ln, err := netx.EscuchaConFallback("127.0.0.1:"+puertoStr, 5)
	if err != nil {
		t.Fatalf("EscuchaConFallback: %v", err)
	}
	defer func() { _ = ln.Close() }()

	_, obtenidoStr, _ := net.SplitHostPort(ln.Addr().String())
	obtenido, _ := strconv.Atoi(obtenidoStr)
	if obtenido == puerto {
		t.Fatal("devolvió el puerto ocupado")
	}
	if obtenido < puerto+1 || obtenido > puerto+5 {
		t.Errorf("puerto %d fuera del rango de fallback [%d,%d]", obtenido, puerto+1, puerto+5)
	}
}

func TestEscuchaConFallbackSeRindeYDevuelveError(t *testing.T) {
	ocupado, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("preparando el puerto ocupado: %v", err)
	}
	defer func() { _ = ocupado.Close() }()

	// Un solo intento sobre un puerto ocupado: no hay a dónde saltar.
	if ln, err := netx.EscuchaConFallback(ocupado.Addr().String(), 1); err == nil {
		_ = ln.Close()
		t.Fatal("quiero error cuando no queda puerto libre")
	}
}

// El puerto 0 significa "el que sea" y ya lo resuelve el sistema: el fallback
// no debe intentar 1, 2, 3...
func TestEscuchaConFallbackRespetaElPuertoCero(t *testing.T) {
	ln, err := netx.EscuchaConFallback("127.0.0.1:0", 3)
	if err != nil {
		t.Fatalf("EscuchaConFallback: %v", err)
	}
	defer func() { _ = ln.Close() }()
	if _, p, _ := net.SplitHostPort(ln.Addr().String()); p == "0" {
		t.Error("el sistema debía asignar un puerto real")
	}
}
