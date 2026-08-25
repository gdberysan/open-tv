import { writable } from 'svelte/store'
import type { ConsultaCatalogo } from '../datos/catalogo'

export type ModoVista = 'rejilla' | 'lista'

export interface EstadoFiltros extends ConsultaCatalogo {
  soloFavoritos: boolean
  vista: ModoVista
}

export const filtros = writable<EstadoFiltros>({
  q: '',
  pais: '',
  categoria: '',
  calidad: '',
  mostrarOffline: false,
  soloFavoritos: false,
  vista: 'rejilla',
})
