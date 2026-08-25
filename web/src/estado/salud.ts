export type ClaseError = 'gateway' | 'red' | 'servidor'

export interface EstadoSalud {
  sincronizando: boolean
  proxyDisponible: boolean
  ultimoSync: Date | null
  version: string
}

/**
 * clasificarError separa tres problemas que se ven igual en pantalla y se
 * arreglan de forma distinta: Open TV cerrado, sin internet, o el servidor
 * contestando mal. El 2026-08-07 el diagnóstico de un "gateway caído" costó
 * una tarde porque el síntoma parecía un bug del cliente.
 */
export function clasificarError(e: unknown): ClaseError {
  const m = e instanceof Error ? e.message : String(e)
  if (m.includes('sin red')) return 'red'
  if (m.includes('gateway')) return 'gateway'
  return 'servidor'
}

export async function consultarSalud(base = ''): Promise<EstadoSalud> {
  const resp = await fetch(`${base}/health`)
  const cuerpo = (await resp.json()) as {
    last_sync?: string | null
    proxy_enabled?: boolean
    version?: string
  }
  return {
    sincronizando: !cuerpo.last_sync,
    proxyDisponible: cuerpo.proxy_enabled === true,
    ultimoSync: cuerpo.last_sync ? new Date(cuerpo.last_sync) : null,
    version: cuerpo.version ?? 'dev',
  }
}
