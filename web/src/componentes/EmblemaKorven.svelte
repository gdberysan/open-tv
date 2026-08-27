<script lang="ts">
  // Emblema de marca de Korven, en SVG inline.
  //
  // Origen: el modelo 3D `korven-emblema-3d.html` (three.js) del proyecto de
  // diseño. Ese fichero NO se puede traer tal cual — carga three.js desde
  // unpkg vía import map y la CSP del cliente pina `script-src 'self'`
  // (internal/ui/ui.go); además serían ~170 KB gzip contra un presupuesto de
  // 80 KB para TODO el bundle propio. Lo que sí se traslada es lo que hacía
  // al modelo reconocible: la MISMA geometría que ya vive en
  // /marca/korven-emblema.svg (hexágono, aristas al vértice, cursor «<»,
  // nodo ámbar + halo) más el SOMBREADO POR FACETA que el modelo obtenía de
  // su luz clave — ahí está el volumen, y cuesta ~1 KB en vez de 170.
  //
  // El reparto de tonos no es decorativo: la luz clave del modelo está en
  // (4, 7, 5) —arriba a la derecha—, así que cada faceta recibe el tono que
  // le tocaría por el coseno del ángulo entre su normal y esa luz. De ahí
  // que el eje 60°–240° (arriba-derecha → abajo-izquierda) sea el que va de
  // más claro a más oscuro, y que el sombreado sea simétrico respecto a él.
  //
  // aria-hidden SIEMPRE, en los dos usos, y a propósito: en la cabecera el
  // wordmark «KORVEN OPEN TV» ya dice quién es, y en el panel de espera lo
  // dice el copy «Elige un canal». Es ornamento junto a texto que ya carga
  // el significado — anunciarlo sería ruido duplicado. Esto además respeta
  // el invariante de a11y del rework: las ÚNICAS regiones anunciadoras
  // nuevas son las dos sr-only persistentes de App.
  let { tamano = 'chico', animado = false }: {
    tamano?: 'chico' | 'grande'
    animado?: boolean
  } = $props()
</script>

<span class="emblema {tamano}" class:animado aria-hidden="true">
  <svg viewBox="0 0 128 128" xmlns="http://www.w3.org/2000/svg" focusable="false">
    <!-- Cuerpo facetado: seis triángulos del centro a cada arista. El tono
         de cada uno es el de la luz clave del modelo (ver cabecera). -->
    <g class="facetas">
      <polygon points="64,64 64,12 110,38" class="f-luz" />
      <polygon points="64,64 110,38 110,90" class="f-media" />
      <polygon points="64,64 110,90 64,116" class="f-sombra" />
      <polygon points="64,64 64,116 18,90" class="f-fondo" />
      <polygon points="64,64 18,90 18,38" class="f-sombra" />
      <polygon points="64,64 18,38 64,12" class="f-media" />
    </g>

    <!-- Aristas del centro al vértice: los `ridge_*` de acero del modelo. -->
    <g class="aristas">
      <path d="M64,64 L64,12 M64,64 L110,38 M64,64 L110,90 M64,64 L64,116 M64,64 L18,90 M64,64 L18,38" />
    </g>

    <!-- Silueta hexagonal: el `rim` de hueso del modelo. NO va a un tono
         plano: en el modelo son seis barras independientes y la luz clave
         las alcanza de forma desigual, así que un contorno uniforme lo
         convierte en una pegatina. Se pinta en dos capas —el hexágono
         entero al tono más bajo, que garantiza la silueta cerrada y sin
         costuras en las esquinas, y encima solo las aristas que la luz
         levanta— con el MISMO reparto que las facetas. -->
    <polygon class="canto" points="64,12 110,38 110,90 64,116 18,90 18,38" />
    <g class="canto-luz">
      <line class="c-luz" x1="64" y1="12" x2="110" y2="38" />
      <line class="c-media" x1="110" y1="38" x2="110" y2="90" />
      <line class="c-media" x1="18" y1="38" x2="64" y2="12" />
      <line class="c-sombra" x1="110" y1="90" x2="64" y2="116" />
      <line class="c-sombra" x1="18" y1="90" x2="18" y2="38" />
    </g>

    <!-- Cursor «<»: el motivo que el modelo repite en las dos caras. -->
    <polyline class="cursor" points="40,52 28,64 40,76" />

    <!-- Nodo ámbar + halo: el `node`/`halo` emisivo, la señal viva. -->
    <circle class="halo" cx="64" cy="64" r="16" />
    <circle class="nodo" cx="64" cy="64" r="9" />
  </svg>
</span>

<style>
  /* perspective en el envoltorio, no en el SVG: el basculado de abajo es un
     giro real en 3D (rotateY), y sin un plano de fuga se vería como un
     simple escalado horizontal. */
  .emblema {
    display: inline-block;
    flex-shrink: 0;
    perspective: 640px;
    line-height: 0;
  }
  .emblema svg {
    display: block;
    width: 100%;
    height: 100%;
  }
  .chico {
    width: 20px;
    height: 20px;
  }
  /* Tamaño óptico: a 20px las seis aristas interiores miden medio píxel y no
     dibujan volumen, solo ensucian el interior. A ese tamaño el emblema lo
     sostienen la silueta y el ojo ámbar, que es justo lo que hay que
     reconocer de un vistazo en la cabecera. */
  .chico .aristas {
    display: none;
  }
  /* El grande vive en el panel de espera (16/9): se escala con el hueco pero
     con topes, para que ni desaparezca en móvil ni domine en pantalla ancha. */
  .grande {
    width: clamp(88px, 18vw, 168px);
    height: clamp(88px, 18vw, 168px);
  }

  /* --- Sombreado por faceta (tokens del ramp graphite, nunca hex crudo) --- */
  .f-luz    { fill: var(--graphite-600); }
  .f-media  { fill: var(--graphite-700); }
  .f-sombra { fill: var(--graphite-750); }
  .f-fondo  { fill: var(--graphite-800); }

  .aristas {
    stroke: var(--graphite-500);
    stroke-width: 2;
    stroke-linejoin: round;
    fill: none;
  }
  /* Capa base del canto: cierra la silueta al tono más bajo del reparto.
     Las opacidades de encima están calculadas para componer SOBRE esta
     (0.38): 1 - (1 - 0.38)(1 - a) da el tono final de cada arista. */
  .canto {
    fill: none;
    stroke: var(--graphite-050);
    stroke-width: 3;
    stroke-linejoin: round;
    opacity: 0.38;
  }
  .canto-luz line {
    stroke: var(--graphite-050);
    stroke-width: 3;
    stroke-linecap: round;
  }
  .canto-luz .c-luz    { opacity: 1; }
  .canto-luz .c-media  { opacity: 0.58; }
  .canto-luz .c-sombra { opacity: 0.2; }
  .cursor {
    fill: none;
    stroke: var(--graphite-200);
    stroke-width: 3;
    stroke-linejoin: round;
    stroke-linecap: round;
  }
  .halo {
    fill: none;
    stroke: var(--amber-500);
    stroke-width: 2;
    opacity: 0.3;
  }
  .nodo {
    fill: var(--amber-500);
  }

  /* --- Movimiento (solo con `animado`; hoy, solo el panel de espera) ---
     `basculacion` es el turntable del modelo recortado a ±14°: el objeto se
     gira, pero nunca llega a ponerse de canto (el cristal es una placa fina
     y de perfil desaparecería). `pulso` es el `emissiveIntensity` del
     modelo —0.5 ± 0.10, o sea ±20%— trasladado a opacidad.
     Duraciones y curvas SIEMPRE de tokens: lo exige movimiento.test.ts. */
  .animado svg {
    animation: basculacion var(--dur-emblema-giro) var(--ease-in-out) infinite alternate;
  }
  .animado .nodo,
  .animado .halo {
    animation: pulso var(--dur-emblema-pulso) var(--ease-in-out) infinite;
  }

  @keyframes basculacion {
    from { transform: rotateY(-14deg) rotateX(3deg); }
    to   { transform: rotateY(14deg) rotateX(-3deg); }
  }
  @keyframes pulso {
    0%, 100% { opacity: 0.8; }
    50%      { opacity: 1; }
  }

  /* Invariante duro del proyecto: prefers-reduced-motion anula TODA
     animación nueva. Al pararse, el emblema se queda en su pose de luz
     clave (sin rotación) y con el ámbar a plena señal — la misma lectura
     estática, sin movimiento ninguno. */
  @media (prefers-reduced-motion: reduce) {
    .animado svg,
    .animado .nodo,
    .animado .halo {
      animation: none;
      transform: none;
    }
    .animado .halo { opacity: 0.3; }
    .animado .nodo { opacity: 1; }
  }
</style>
