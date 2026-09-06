// Package stats agrega los desenlaces de reproducción que el cliente reporta.
// Todo vive en memoria y se expone SOLO localmente: es observabilidad sin
// rastreo — nada sale de la máquina (el pie de la app dice "Korven no
// retransmite nada", y aquí se cumple).
package stats

import "sync"

// Desenlace es un evento de reproducción reportado por el cliente.
type Desenlace struct {
	CanalID       string
	Resultado     string // "iniciado" | "fallo" | "cortado"
	Motivo        string
	Motor         string // "nativo" | "hlsjs"
	Via           string // "directo" | "proxy" | "ninguna" (sin intento: todos los mirrors indecodificables)
	MirrorIndex   int
	MsPrimerFrame int
}

// Resumen es la vista agregada que expone GET /stats.
type Resumen struct {
	Intentos  int            `json:"intentos"`
	Iniciados int            `json:"iniciados"`
	Fallos    int            `json:"fallos"`
	Cortados  int            `json:"cortados"`
	TasaExito float64        `json:"tasa_exito"`
	PorMotivo map[string]int `json:"por_motivo"`
	PorVia    map[string]int `json:"por_via"`
	PorMotor  map[string]int `json:"por_motor"`
}

// Agregador acumula desenlaces de forma segura entre goroutines.
type Agregador struct {
	mu        sync.Mutex
	intentos  int
	iniciados int
	fallos    int
	cortados  int
	porMotivo map[string]int
	porVia    map[string]int
	porMotor  map[string]int
}

func NuevoAgregador() *Agregador {
	return &Agregador{
		porMotivo: map[string]int{},
		porVia:    map[string]int{},
		porMotor:  map[string]int{},
	}
}

func (a *Agregador) Registrar(d Desenlace) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.intentos++
	switch d.Resultado {
	case "iniciado":
		a.iniciados++
	case "fallo":
		a.fallos++
	case "cortado":
		a.cortados++
	}
	if d.Motivo != "" {
		a.porMotivo[d.Motivo]++
	}
	if d.Via != "" {
		a.porVia[d.Via]++
	}
	if d.Motor != "" {
		a.porMotor[d.Motor]++
	}
}

func (a *Agregador) Resumen() Resumen {
	a.mu.Lock()
	defer a.mu.Unlock()
	var tasa float64
	if a.intentos > 0 {
		tasa = float64(a.iniciados) / float64(a.intentos)
	}
	return Resumen{
		Intentos: a.intentos, Iniciados: a.iniciados, Fallos: a.fallos, Cortados: a.cortados,
		TasaExito: tasa,
		PorMotivo: copiaMapa(a.porMotivo), PorVia: copiaMapa(a.porVia), PorMotor: copiaMapa(a.porMotor),
	}
}

func copiaMapa(m map[string]int) map[string]int {
	c := make(map[string]int, len(m))
	for k, v := range m {
		c[k] = v
	}
	return c
}
