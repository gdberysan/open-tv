# Reproductor primero — layout de reproducción persistente (artboard 1b)

**Estado:** diseño aprobado en brainstorming (2026-08-26). Autoridad para el plan.
**Referencia visual:** artboard `1b · Reproductor primero` de `open-tv-interface-design/…/Open TV - Exploraciones de IA.dc.html` (mockup estático; esta spec lo lleva al cliente Svelte real).

## 1. Contexto y objetivo

Hoy el cliente es **catálogo-primero**: la rejilla es la vista principal y el
reproductor es un MODAL (`role="dialog"`, focus-trap, fondo `inert`) que se abre
encima y se cierra. El artboard 1b invierte el modelo: **se entra viendo** — el
vídeo vive persistente a la izquierda y el catálogo se colapsa en una barra
lateral a la derecha; cambiar de canal **intercambia el vídeo en el sitio**, sin
modal ni volver a una pantalla de rejilla.

**Decisiones de brainstorming (cerradas):**
1. **Player-first por defecto, PERO la rejilla rica de P0.6 se conserva** a un
   clic como modo «ver todo» (no se retira la «sala de control»).
2. **Entrada:** al arrancar, si hay historial, el panel de vídeo **auto-reproduce
   el último canal EN SILENCIO** (los navegadores bloquean autoplay con sonido)
   con un **toque para activar el sonido** bien visible. Instalación vacía →
   onboarding manda; con fuentes pero sin historial → tarjeta «Continuar viendo»/
   elige un canal, sin auto-reproducir.

## 2. Tres estados de nivel superior de la app

```
sinFuentes (P0.7)            → <Onboarding/> ocupa la página (sin panel de vídeo)
modoVerTodo (toggle)        → rejilla completa (BarraLateralFacetas + RejillaVirtual, como hoy)
por defecto (con catálogo)  → ESCENARIO reproductor-primero (abajo)
```
Overlays por encima de cualquiera: la paleta ⌘K y las vistas-hash
`#ajustes`/`#fuentes`/`#stats`/`#grabaciones` (siguen siendo overlays con
focus-trap, ver §6).

## 3. El escenario reproductor-primero

Layout de dos paneles (desktop): `grid-template-columns: 1fr <ancho-catalogo>`
(el 1b usa 372px; token/variable, no mágico).

- **Panel de vídeo (1fr, izquierda):** el `Reproductor` PERSISTENTE reproduciendo
  `canalActual`, con TODO su chrome de overlay ya existente (insignia ● En vivo,
  meta `resolución · ms · país`, nombre, Silenciar/★Favorito/Pantalla-completa/
  PiP, la línea now/next de EPG, y —futuro— el ● Grabar del DVR), el failover +
  CTA mirror, y el `PlaybackGuard`. Debajo del vídeo: la tira «Continuar viendo»
  (historial), como en el 1b.
- **Barra lateral de catálogo (derecha):** buscador «Buscar un canal», **facetas
  compactas** (chips de país/género con conteos — versión condensada de
  `BarraLateralFacetas`, no el panel ancho), y la **lista de canales
  virtualizada con salud** (punto+ms+resolución, `SenalCanal`), con el canal en
  curso marcado «Reproduciendo». Reutiliza la virtualización de P0.5 (8.8k
  canales en una lista estrecha).
- **Cambio de canal EN EL SITIO:** clicar un canal en la lateral (o en el modo
  ver-todo) hace `canalActual = canal` — el reproductor persistente recarga sin
  abrir/cerrar un modal, «sin interrumpir la vista».

## 4. Refactor del `Reproductor`: de modal a panel persistente (la pieza delicada)

- Hoy `Reproductor.svelte` es un modal (`role="dialog"`, `aria-modal`,
  focus-trap sobre `contenedorDialogo`, cierre por Esc/botón, fondo `inert`). Se
  refactoriza para funcionar como **panel persistente**: sin `aria-modal`, sin
  atrapar el foco por defecto (el vídeo y la lateral coexisten en el orden de
  tabulación), sin «cerrar» (siempre hay un canal en curso una vez hay catálogo).
- **Se CONSERVA intacta** la maquinaria interna: `planDeFailover` + CTA «Probar el
  siguiente mirror», `PlaybackGuard`, silenciado, PiP (P0.8), la línea EPG (P2) y
  su refresco, el auto-ocultar del overlay. Cambian el ENVOLTORIO (modal→panel) y
  el modelo de foco, no el motor de reproducción.
- **Pantalla completa:** hoy `requestFullscreen` sobre `contenedorDialogo`; ahora
  sobre el panel de vídeo. En pantalla completa el overlay sigue visible (mismo
  patrón que P0.8). Esc sale de pantalla completa (no «cierra», porque ya no hay
  modal que cerrar) — precedencia: si `document.fullscreenElement`, salir; si no,
  Esc no hace nada en el escenario por defecto (sí cierra overlays reales).
- **Cambio de canal:** el efecto que hoy depende de `canal.id` (recarga hls, pide
  EPG) sigue igual — solo que `canal` viene de `canalActual`, no de un modal que
  se abre.

## 5. Entrada / autoplay (muted + tap-to-unmute)

- Al montar el escenario con historial: `canalActual = últimoDelHistorial`, el
  `<video>` arranca `muted autoplay` (cumple la política del navegador), y se
  muestra una affordance clara **«Toca para activar el sonido»** (un botón grande
  sobre el vídeo o el propio control Silenciar resaltado). Primer gesto del
  usuario → `video.muted = false`.
- Con fuentes pero SIN historial: el panel de vídeo muestra la tarjeta «Continuar
  viendo» vacía → «elige un canal para empezar» (sin auto-reproducir); la lateral
  invita a elegir.
- Sin fuentes: onboarding ocupa la página; no hay panel de vídeo.
- Honestidad: el estado de señal sigue siendo honesto (si el último canal está
  caído, el failover actúa y, si ninguno reproduce, el mensaje de fallo por
  clase, como P0.5) — nunca un panel negro mudo sin explicación.

## 6. Modelo de foco / inert (rework)

- **Por defecto (escenario):** NO hay `inert` de fondo. El panel de vídeo (sus
  controles) y la barra lateral (buscador, chips, lista con roving-tabindex)
  están AMBOS en el orden de tabulación natural. No hay trap.
- **Overlays reales** (paleta ⌘K, `#ajustes`/`#fuentes`/`#stats`/`#grabaciones`,
  y el modo ver-todo si se decide modal): SIGUEN con fondo `inert` + focus-trap +
  focus-restore, exactamente como P0.6/P0.8. Se conserva el invariante «2
  regiones sr-only persistentes como únicos anunciadores» y «un solo overlay
  atrapando a la vez».
- **⌘K** ya no se inhibe «con el reproductor abierto» (ya no hay modal
  reproductor); se inhibe solo con otro overlay abierto. Ajustar el guard.

## 7. Modo «ver todo» (rejilla completa, P0.6 conservada)

- Un control (en la cabecera o sobre la lateral) abre la **rejilla rica completa**
  (la actual `BarraLateralFacetas` ancha + `RejillaVirtual` con tarjetas/logos).
  Es la misma experiencia de hoy, ahora como modo secundario. Seleccionar un
  canal ahí hace `canalActual = canal` y vuelve al escenario reproduciéndolo.
- Implementación: reutiliza los componentes existentes; es un modo/estado de App,
  no código nuevo de catálogo. Decisión del plan: overlay a pantalla completa vs
  reemplazo del escenario (recomendado: reemplazo, con retorno claro).

## 8. Responsive

- **Ancho (desktop):** dos paneles (vídeo 1fr | catálogo lateral).
- **Estrecho (móvil/ventana pequeña):** vídeo arriba (16:9), catálogo debajo
  (lista) o en cajón, reutilizando el patrón de cajón responsive de P0.6
  (`esEstrecho`, `boton-cajon`, `inert` del cajón cerrado). El vídeo NO se pierde
  al navegar la lista.

## 9. Restricciones (Global Constraints del plan)

- **`mobile/` CERO diffs;** contrato JSON `/channels`+Flutter intacto (esto es
  puro cliente; el backend no cambia).
- **Ninguna dependencia nueva.** **Bundle propio ≤ 80 KB gzip** (es
  reestructuración, no librería nueva; vigilar que no crezca).
- **Tokens de marca** ya adoptados (ámbar=activo/foco; el 1b usa el mismo design
  system). Nada hardcodeado.
- **a11y (el punto más delicado):** el modelo de foco se REESTRUCTURA (§6) sin
  perder los invariantes P0.6/P0.8 — 2 regiones sr-only persistentes, roving
  tabindex en la lista, overlays reales atrapan/restauran, `prefers-reduced-motion`
  respetado. La transición vídeo↔lista debe ser navegable por teclado y anunciada
  con honestidad. **Gate VoiceOver del autor OBLIGATORIO** (cambia el foco de
  toda la app).
- **i18n ES (fuente) + EN paridad** para cadenas nuevas (p. ej. «toca para activar
  el sonido», «ver todo», «reproduciendo»).
- **Gates** de los 3 lenguajes + golangci-lint (aunque casi todo es web) +
  scrubcheck. Identidad `gdberysan@gmail.com` + trailer.

## 10. Pruebas y gates

- **Web (unit/integración):** el escenario monta con vídeo persistente + lateral;
  clicar un canal cambia `canalActual` SIN montar un modal (el vídeo no se
  desmonta/remonta); autoplay muted + tap-to-unmute (con `video.muted` mockeado);
  sin historial → prompt, no autoplay; sin fuentes → onboarding; el modo ver-todo
  abre la rejilla y seleccionar vuelve reproduciendo; ⌘K y las vistas-hash siguen
  atrapando el foco; la maquinaria del reproductor (failover/PiP/EPG) sigue verde.
- **a11y:** el orden de tabulación en el escenario (vídeo→lateral); overlays
  atrapan/restauran; sin regiones aria-live nuevas; roving en la lista.
- **e2e Playwright:** arrancar con un catálogo de fixture → el escenario aparece
  reproduciendo el último (muted) → cambiar de canal en la lateral swap-in-place
  → abrir ver-todo y volver → contra el binario real, 3 motores.
- **`mobile/` cero diffs** por tarea. **Gate manual VoiceOver + Safari/Firefox**
  del autor tras el build (obligatorio por el cambio de foco).

## 11. No-objetivos (fuera de esta spec)

- Los otros artboards del archivo de diseño (1a, 1c «filas con criterio», etc.) —
  cada uno sería su propia decisión.
- Cambios de backend (ninguno).
- DVR / grabación (spec aparte, `2026-08-26-dvr-grabacion-local-design.md`) —
  su ● Grabar vivirá en el reproductor persistente que ESTA spec crea, así que
  este layout va PRIMERO y el DVR se apoya en él.
- Timeshift, casting (specs propias).

## 12. Riesgos

| Riesgo | Mitigación |
|---|---|
| Refactor del `Reproductor` (modal→panel) rompe failover/PiP/PlaybackGuard/EPG | Se conserva el motor intacto; solo cambia el envoltorio y el foco. Tests de esas piezas (P0.5/P0.8/P2) deben seguir verdes; revisión por-tarea + gate Chrome. |
| El nuevo modelo de foco degrada la a11y | Invariantes P0.6/P0.8 explícitos en cada tarea; gate VoiceOver OBLIGATORIO; overlays reales conservan trap/restore. |
| Autoplay con sonido bloqueado por el navegador confunde | muted-autoplay + affordance «toca para activar el sonido» clara y probada; nunca audio por sorpresa. |
| Perder la rejilla rica de P0.6 | NO se pierde: modo «ver todo» la conserva a un clic; el escenario es un modo nuevo, no un reemplazo destructivo. |
| Regresión de bundle por reestructurar | Vigilar ≤80KB; reutilizar componentes existentes (lista, facetas, reproductor), no reescribir. |
| Lista estrecha con 8.8k canales | Reutiliza la virtualización de P0.5 (ya probada a esa escala). |
