package acceso

import (
	"sync"
	"time"
)

// Limitador cuenta FALLOS de acceso por IP en ventanas fijas. Con una clave
// de 256 bits la fuerza bruta no es viable de todos modos; esto existe para que
// un script no llene el log ni el procesador. Solo cuentan los intentos con
// clave incorrecta: un login correcto nunca consume cupo, así que varios
// dispositivos detrás del mismo NAT pueden entrar cada uno el suyo sin
// pisarse.
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

// Agotado dice si clave (la IP) ya tiene max fallos dentro de la ventana
// actual. No registra nada por sí solo: solo mira.
func (l *Limitador) Agotado(clave string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.limpiar()
	c, ok := l.cuentas[clave]
	if !ok {
		return false
	}
	return c.intentos >= l.max
}

// Fallo registra un intento fallido de clave; abre una ventana nueva si la
// anterior ya expiró.
func (l *Limitador) Fallo(clave string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.limpiar()
	c := l.cuentas[clave]
	if c.intentos == 0 {
		c.desde = l.ahora()
	}
	c.intentos++
	l.cuentas[clave] = c
}

// limpieza perezosa: el mapa no crece con IPs cuya ventana ya expiró. Se
// llama con l.mu ya tomado.
func (l *Limitador) limpiar() {
	ahora := l.ahora()
	for k, c := range l.cuentas {
		if ahora.Sub(c.desde) >= l.ventana {
			delete(l.cuentas, k)
		}
	}
}
