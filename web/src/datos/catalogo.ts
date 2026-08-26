/** Canal tal y como lo usa el cliente. Nombres propios, no los del cable. */
export interface Canal {
  id: string
  nombre: string
  logoUrl: string
  categoriaId: string
  idioma: string
  pais: string
  /** null = ningún stream comprobado aún. NO es lo mismo que false. */
  vivo: boolean | null
  latenciaMs: number
  /** Veredicto estricto (hls.js). null = sin comprobar. */
  webOk: boolean | null
}

export interface Faceta {
  valor: string
  total: number
}

export interface ConsultaCatalogo {
  q?: string
  pais?: string
  categoria?: string
  calidad?: string
  mostrarOffline?: boolean
  /** Filtro de favoritos. Un array VACÍO significa "ninguno", no "sin filtro". */
  ids?: string[]
  limite?: number
  desplazamiento?: number
}

export interface PaginaCanales {
  canales: Canal[]
  total: number
}

export interface Frescura {
  /** 'vivo' = gateway local; 'instantanea' = snapshot estático (P3). */
  tipo: 'vivo' | 'instantanea'
  generadoEn: Date | null
}

export interface DestinoStream {
  url: string
  airplayOk: boolean | null
}

/** Un mirror de un canal con su salud, para el failover. */
export interface Mirror {
  url: string
  vivo: boolean | null
  latenciaMs: number
  webOk: boolean | null
}

/**
 * CatalogSource es la costura entre el gateway vivo y el snapshot estático.
 * El cliente no sabe en cuál está salvo por la línea de frescura.
 */
export interface CatalogSource {
  canales(c: ConsultaCatalogo): Promise<PaginaCanales>
  paises(): Promise<Faceta[]>
  categorias(): Promise<Faceta[]>
  calidades(): Promise<Faceta[]>
  aleatorio(c: ConsultaCatalogo): Promise<Canal>
  destino(id: string): Promise<DestinoStream>
  /** Los mirrors del canal, ordenados por salud (vivo y menor latencia primero). */
  mirrors(id: string): Promise<Mirror[]>
  frescura(): Promise<Frescura>
  /** El proxy solo existe en el binario local. */
  proxyDisponible(): Promise<boolean>
}
