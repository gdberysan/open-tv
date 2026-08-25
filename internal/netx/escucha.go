// Package netx contiene el poco código de red que no encaja en un adaptador.
package netx

import (
	"fmt"
	"net"
	"strconv"
)

// EscuchaConFallback abre un listener en addr y, si el puerto está ocupado,
// prueba los intentos-1 siguientes.
//
// Un binario de escritorio no puede rendirse porque el 8080 esté cogido: la
// mitad de las herramientas de desarrollo lo usan. Con puerto 0 no hay
// fallback que valga — el sistema ya elige uno libre.
func EscuchaConFallback(addr string, intentos int) (net.Listener, error) {
	host, puertoStr, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("netx.EscuchaConFallback (%q): %w", addr, err)
	}
	puerto, err := strconv.Atoi(puertoStr)
	if err != nil {
		return nil, fmt.Errorf("netx.EscuchaConFallback (puerto %q): %w", puertoStr, err)
	}
	if intentos < 1 {
		intentos = 1
	}
	if puerto == 0 {
		intentos = 1
	}

	var ultimo error
	for i := 0; i < intentos; i++ {
		candidato := net.JoinHostPort(host, strconv.Itoa(puerto+i))
		ln, err := net.Listen("tcp", candidato)
		if err == nil {
			return ln, nil
		}
		ultimo = err
	}
	return nil, fmt.Errorf("netx.EscuchaConFallback: ningún puerto libre entre %d y %d: %w",
		puerto, puerto+intentos-1, ultimo)
}
