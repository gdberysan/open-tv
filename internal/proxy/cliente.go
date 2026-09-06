package proxy

import (
	"fmt"
	"net"
	"net/http"
	"syscall"
)

// guardiaRed son las dos defensas de red del proxy, separadas del Handler
// para que cualquier fetch server-side de streams (la sonda de códecs del
// health-check, por ejemplo) las reutilice tal cual: el mismo cliente, no una
// copia que se desvíe con el tiempo.
type guardiaRed struct {
	// privadasOK solo es true en tests: httptest vive en 127.0.0.1, que en
	// producción es exactamente lo que hay que bloquear.
	privadasOK bool
}

// NuevoClienteGuardado construye el cliente HTTP con el que el proxy relaya
// y con el que se hace cualquier petición a una URL dictada por un tercero
// (un manifiesto). Sin timeout de cliente: lo pone el contexto de cada
// petición. Sin keep-alive y con 4 conexiones por host, como el checker.
func NuevoClienteGuardado(permitirDestinosPrivados bool) *http.Client {
	g := guardiaRed{privadasOK: permitirDestinosPrivados}
	dialer := &net.Dialer{
		Timeout: tiempoPeticion,
		// Control se ejecuta con la IP YA resuelta, justo antes de que el
		// kernel abra la conexión — no la que destinoPrivado comprobó antes
		// de que el propio Transport volviera a resolver el hostname por su
		// cuenta. Sin esto, un DNS que cambie de respuesta entre esa
		// comprobación y la conexión real (rebinding) esquiva el filtro por
		// hostname.
		Control: g.controlConexion,
	}
	return &http.Client{
		Transport: &http.Transport{
			MaxConnsPerHost:   4,
			DisableKeepAlives: true,
			DialContext:       dialer.DialContext,
		},
		// El http.Client por defecto sigue redirecciones (hasta 10) sin
		// preguntar. Un origen que pasó el filtro puede responder 302 hacia
		// 127.0.0.1 o hacia el enlace-local de metadatos de una nube.
		CheckRedirect: g.checkRedirect,
	}
}

// checkRedirect se ejecuta en cada salto de una redirección 3xx, ANTES de que
// el cliente la siga.
func (g guardiaRed) checkRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= maxRedirecciones {
		return fmt.Errorf("demasiadas redirecciones (%d)", len(via))
	}
	if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
		return fmt.Errorf("esquema no permitido en redirección: %s", req.URL.Scheme)
	}
	if !g.privadasOK && destinoPrivado(req.Context(), req.URL.Hostname()) {
		return fmt.Errorf("redirección a destino no permitido: %s", req.URL.Hostname())
	}
	return nil
}

// controlConexion se ejecuta justo antes de que el sistema operativo abra la
// conexión TCP, con la dirección YA resuelta. Es la única comprobación que
// mira la IP con la que el kernel conecta de verdad, así que es la que de
// verdad cierra el DNS-rebinding.
func (g guardiaRed) controlConexion(_, address string, _ syscall.RawConn) error {
	if g.privadasOK {
		return nil
	}
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("dirección de conexión inválida: %w", err)
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return fmt.Errorf("dirección de conexión no es una IP: %s", host)
	}
	if ipPrivada(ip) {
		return fmt.Errorf("conexión bloqueada a destino privado: %s", ip)
	}
	return nil
}
