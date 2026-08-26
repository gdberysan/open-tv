package stats_test

import (
	"testing"

	"github.com/gdberysan/open-tv/internal/stats"
)

func TestAgregadorCuentaYclasifica(t *testing.T) {
	a := stats.NuevoAgregador()
	a.Registrar(stats.Desenlace{Resultado: "iniciado", Motor: "hlsjs", Via: "proxy"})
	a.Registrar(stats.Desenlace{Resultado: "iniciado", Motor: "nativo", Via: "directo"})
	a.Registrar(stats.Desenlace{Resultado: "fallo", Motivo: "timeout-de-carga", Motor: "hlsjs", Via: "directo"})

	r := a.Resumen()
	if r.Intentos != 3 || r.Iniciados != 2 || r.Fallos != 1 {
		t.Fatalf("resumen = %+v", r)
	}
	if r.TasaExito < 0.66 || r.TasaExito > 0.67 {
		t.Errorf("tasa_exito = %v, quiero ~0.666", r.TasaExito)
	}
	if r.PorMotivo["timeout-de-carga"] != 1 {
		t.Errorf("por_motivo = %v", r.PorMotivo)
	}
	if r.PorVia["proxy"] != 1 || r.PorVia["directo"] != 2 {
		t.Errorf("por_via = %v", r.PorVia)
	}
}

// Concurrente: el cliente puede reportar desde varios reproductores; -race debe
// quedar limpio.
func TestAgregadorEsSeguroConcurrentemente(t *testing.T) {
	a := stats.NuevoAgregador()
	done := make(chan struct{})
	for i := 0; i < 8; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				a.Registrar(stats.Desenlace{Resultado: "iniciado"})
			}
			done <- struct{}{}
		}()
	}
	for i := 0; i < 8; i++ {
		<-done
	}
	if a.Resumen().Intentos != 800 {
		t.Errorf("intentos = %d, quiero 800", a.Resumen().Intentos)
	}
}
