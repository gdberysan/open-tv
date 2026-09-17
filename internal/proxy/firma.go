package proxy

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"
)

// FicheroClaveProxy vive en el directorio de datos, junto a access-key.
const FicheroClaveProxy = "proxy-key"

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

// NuevoFirmadorPersistente es el de producción: la clave se guarda en ruta y
// sobrevive a los reinicios. Tiene que sobrevivir: hls.js reintenta las URLs
// hijas firmadas (playlists de nivel, segmentos) que ya tiene, sin volver a
// pedir el manifiesto, así que con una clave por proceso un docker restart o
// un respawn de launchd dejaba la reproducción en curso colgada con 403.
func NuevoFirmadorPersistente(ruta string) (*Firmador, error) {
	datos, err := os.ReadFile(ruta) //nolint:gosec // ruta = directorio de datos propio + nombre fijo
	if err == nil {
		clave, errDec := base64.RawURLEncoding.DecodeString(strings.TrimSpace(string(datos)))
		if errDec != nil || len(clave) < 32 {
			return nil, fmt.Errorf("proxy: %s no contiene una clave válida (base64url de al menos 32 bytes); bórralo para generar otra", ruta)
		}
		return &Firmador{clave: clave}, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("proxy: leyendo %s: %w", ruta, err)
	}
	clave := make([]byte, 32)
	if _, err := rand.Read(clave); err != nil {
		return nil, fmt.Errorf("proxy: generando la clave: %w", err)
	}
	if err := os.WriteFile(ruta, []byte(base64.RawURLEncoding.EncodeToString(clave)+"\n"), 0o600); err != nil {
		return nil, fmt.Errorf("proxy: guardando %s: %w", ruta, err)
	}
	return &Firmador{clave: clave}, nil
}

// NuevoFirmadorAleatorio da una clave nueva por proceso. Solo sirve donde las
// firmas no tienen que sobrevivir a un reinicio: el valor por defecto del
// router (pruebas) cuando nadie le pasa un firmador.
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
