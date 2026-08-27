import type { DesenlaceReproduccion } from '../reproductor/failover'

// reportarDesenlace manda el desenlace a /stats/playback. Best-effort: nunca
// lanza ni bloquea la reproducción — las estadísticas no valen una excepción
// en la UI. Todo local (mismo origen), nada sale de la máquina.
export function reportarDesenlace(o: DesenlaceReproduccion, base = ''): void {
  try {
    const body = JSON.stringify({
      canal_id: o.canalId,
      resultado: o.resultado,
      motivo: o.motivo ?? '',
      motor: o.motor,
      via: o.via,
      mirror_index: o.mirrorIndex,
      // Redondeado: performance.now() trae decimales y un backend que
      // decodifique a int rechazaría el cuerpo entero con 400 (bug real:
      // los desenlaces 'iniciado' se perdían). Milisegundos enteros bastan.
      ms_primer_frame: Math.round(o.msPrimerFrame ?? 0),
    })
    void fetch(`${base}/stats/playback`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body })
      .catch(() => {})
  } catch {
    // Ni un fallo de serialización ni de transporte rompe la reproducción.
  }
}
