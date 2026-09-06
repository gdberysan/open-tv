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
  /** Veredicto de la sonda del primer segmento: false = ningún navegador
   *  decodifica su vídeo (el failover lo salta). null/undefined = sin
   *  sondear. */
  codecOk?: boolean | null
  /** Cadena corta ("mpeg2video,mp2") para el mensaje; '' si no se sabe. */
  codecs?: string
  /** Veredicto de la sonda de audio del primer segmento. null/undefined =
   *  sin sondear, igual que codecOk. */
  audioOk?: boolean | null
  /** Tiempo en ms hasta el primer frame decodificado, medido server-side.
   *  0 = sin medición aún. */
  imagenMs?: number
  /** true = ningún intento reciente llegó a dar imagen. */
  sinImagen?: boolean
  /** Segundos desde el último fallo registrado; 0 = sin fallos o desconocido. */
  ultimoFalloHaceS?: number
}

/** Un programa de la guía EPG. inicioSeg/finSeg: epoch UTC en SEGUNDOS, tal
 * cual los manda el cable (nombres propios para que quede claro que NO son
 * milisegundos, a diferencia del resto del cliente). */
export interface Programa {
  titulo: string
  inicioSeg: number
  finSeg: number
}

/** "Ahora" y "siguiente" de un canal. Cualquiera de los dos puede ser null
 * (p.ej. fuera de horario de emisión) sin que eso signifique "sin guía": un
 * canal con tvg_id pero sin programa en curso ni futuro sigue teniendo
 * entrada — la ausencia total de guía se modela con la clave ausente del
 * Map que devuelve epgDeCanales, no con este tipo. */
export interface AhoraDespues {
  ahora: Programa | null
  siguiente: Programa | null
}

/** Una fuente de canales añadida por el usuario (bring-your-own). */
export interface Fuente {
  id: string
  label: string
  url: string
  kind: 'url' | 'file'
  /** null = nunca sincronizada. El cable manda 0 para ese caso. */
  ultimoSync: number | null
  canales: number
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
  /** Tiempo-hasta-la-imagen por canal, EN LOTE. Endpoint nuevo (Task 1-4);
   *  opcional para no romper dobles de test que no lo implementan — el
   *  llamador usa `fuente.imagen?.()`. */
  imagen?(): Promise<Record<string, { imagenMs: number; sinImagen: boolean }>>
  frescura(): Promise<Frescura>
  /** El proxy solo existe en el binario local. */
  proxyDisponible(): Promise<boolean>
  /** Las fuentes añadidas por el usuario (bring-your-own). */
  fuentes(): Promise<Fuente[]>
  /** Añade una fuente por URL remota. */
  anadirFuente(url: string, label?: string): Promise<Fuente>
  /** Añade una fuente subiendo un fichero local (multipart). */
  anadirFuenteFichero(f: File): Promise<Fuente>
  /** Quita una fuente existente. */
  quitarFuente(id: string): Promise<void>
  /** Fuerza una resincronización de una fuente existente. */
  resyncFuente(id: string): Promise<void>
  /** Fuentes de ejemplo sugeridas para dar de alta rápido. */
  fuentesSugeridas(): Promise<{ label: string; url: string }[]>
  /** Ahora/después en LOTE para varios canales (la vista de catálogo). Un id
   * sin guía posible (sin tvg_id en ninguna fuente) NO aparece como clave del
   * Map — ausente = sin guía, no un AhoraDespues con ambos campos a null. */
  epgDeCanales(ids: string[]): Promise<Map<string, AhoraDespues>>
  /** Ahora + los próximos programas de UN canal (el overlay). */
  epgDeCanal(id: string, limite?: number): Promise<{ ahora: Programa | null; proximos: Programa[] }>
}
