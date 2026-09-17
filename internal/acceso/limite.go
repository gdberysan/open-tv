package acceso

import (
	"sync"
	"time"
)

// Limitador cuenta intentos de acceso por IP en ventanas fijas. Con una clave
// de 256 bits la fuerza bruta no es viable de todos modos; esto existe para que
// un script no llene el log ni el procesador.
type Limitador struct {
	mu      sync.Mutex
	max     int
	ventana time.Duration
	ahora   func() time.Time
	cuentas map[string]cuenta
}

type cuenta struct {
	desde    time.Time
	intentos int
}

func NuevoLimitador(max int, ventana time.Duration, ahora func() time.Time) *Limitador {
	if ahora == nil {
		ahora = time.Now
	}
	return &Limitador{max: max, ventana: ventana, ahora: ahora, cuentas: make(map[string]cuenta)}
}

func (l *Limitador) Permitir(clave string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	ahora := l.ahora()
	// Limpieza perezosa: el mapa no crece con IPs que ya no intentan nada.
	for k, c := range l.cuentas {
		if ahora.Sub(c.desde) >= l.ventana {
			delete(l.cuentas, k)
		}
	}
	c := l.cuentas[clave]
	if c.intentos == 0 {
		c.desde = ahora
	}
	if c.intentos >= l.max {
		return false
	}
	c.intentos++
	l.cuentas[clave] = c
	return true
}
