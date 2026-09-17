package proxy

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

// Firmador firma las URLs que el propio proxy emite al reescribir un
// manifiesto. Sin firma, /proxy/hls?u= relayaría cualquier URL pública que se
// le pidiera: un relé abierto con la IP de quien lo levanta. Con ella, solo
// pasan las URLs del catálogo (comprobadas aparte) y las que este mismo
// proceso sacó de un manifiesto que ya había autorizado.
type Firmador struct {
	clave []byte
}

// NuevoFirmador copia la clave: quien la pasa no puede cambiarla por debajo.
func NuevoFirmador(clave []byte) (*Firmador, error) {
	if len(clave) < 32 {
		return nil, fmt.Errorf("proxy.NuevoFirmador: la clave necesita al menos 32 bytes, tiene %d", len(clave))
	}
	c := make([]byte, len(clave))
	copy(c, clave)
	return &Firmador{clave: c}, nil
}

// NuevoFirmadorAleatorio es el de producción: una clave nueva por proceso. Al
// reiniciar, las URLs hijas ya emitidas dejan de valer, y el reproductor las
// recupera volviendo a pedir el manifiesto, que es del catálogo.
func NuevoFirmadorAleatorio() (*Firmador, error) {
	clave := make([]byte, 32)
	if _, err := rand.Read(clave); err != nil {
		return nil, fmt.Errorf("proxy.NuevoFirmadorAleatorio: %w", err)
	}
	return &Firmador{clave: clave}, nil
}

// Firmar devuelve el HMAC-SHA256 de u en base64url sin relleno, que viaja tal
// cual en una query sin escapar.
func (f *Firmador) Firmar(u string) string {
	mac := hmac.New(sha256.New, f.clave)
	_, _ = mac.Write([]byte(u))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// Valida compara en tiempo constante. Un firmador nil no valida nada: el
// proxy sin firmador solo acepta lo que esté en el catálogo.
func (f *Firmador) Valida(u, firma string) bool {
	if f == nil || firma == "" {
		return false
	}
	return hmac.Equal([]byte(f.Firmar(u)), []byte(firma))
}
