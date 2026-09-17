package acceso

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// DuracionSesion: un televisor o un móvil de casa no deberían pedir la clave
// cada semana.
const DuracionSesion = 30 * 24 * time.Hour

// Sesiones emite y valida tokens sin estado: v1.<caduca>.<nonce>.<mac>. El
// secreto se deriva de la clave, así que cambiarla invalida todas las sesiones
// sin guardar nada en disco.
type Sesiones struct {
	secreto []byte
	resumen [32]byte
	ahora   func() time.Time
}

func NuevasSesiones(clave string, ahora func() time.Time) *Sesiones {
	if ahora == nil {
		ahora = time.Now
	}
	mac := hmac.New(sha256.New, []byte(clave))
	_, _ = mac.Write([]byte("open-tv/sesion/v1"))
	return &Sesiones{secreto: mac.Sum(nil), resumen: sha256.Sum256([]byte(clave)), ahora: ahora}
}

// ClaveCorrecta compara resúmenes de longitud fija en tiempo constante: ni el
// contenido ni la longitud de la clave se filtran por el tiempo de respuesta.
func (s *Sesiones) ClaveCorrecta(intento string) bool {
	r := sha256.Sum256([]byte(intento))
	return subtle.ConstantTimeCompare(r[:], s.resumen[:]) == 1
}

func (s *Sesiones) Emitir() (string, error) {
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("acceso: generando el nonce: %w", err)
	}
	caduca := strconv.FormatInt(s.ahora().Add(DuracionSesion).Unix(), 10)
	cuerpo := "v1." + caduca + "." + base64.RawURLEncoding.EncodeToString(nonce)
	return cuerpo + "." + s.firmar(cuerpo), nil
}

func (s *Sesiones) Valida(token string) bool {
	partes := strings.Split(token, ".")
	if len(partes) != 4 || partes[0] != "v1" {
		return false
	}
	cuerpo := strings.Join(partes[:3], ".")
	if !hmac.Equal([]byte(s.firmar(cuerpo)), []byte(partes[3])) {
		return false
	}
	caduca, err := strconv.ParseInt(partes[1], 10, 64)
	if err != nil {
		return false
	}
	return s.ahora().Unix() < caduca
}

func (s *Sesiones) firmar(cuerpo string) string {
	mac := hmac.New(sha256.New, s.secreto)
	_, _ = mac.Write([]byte(cuerpo))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
