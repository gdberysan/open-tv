# Diseño — Verificación honesta de la reproducción

> **Fecha:** 2026-09-05 · **Estado:** pendiente de revisión del dueño
> **Origen:** la sesión del 2026-09-04. Cuatro bugs de cara al usuario, dos
> Critical, y las 501 pruebas en verde durante todos ellos.

---

## 1. Objetivo

Que el proyecto sepa **por sí solo** cuándo la reproducción está rota.

Hoy no lo sabe. En una sola sesión, con la suite entera en verde:

| Lo que el usuario vio | Lo que pasaba de verdad |
|---|---|
| «La dirección del canal caducó. Hay 2 mirrors con mejor salud» | Ninguna de las tres afirmaciones era cierta |
| El botón «Reintentar» no hacía nada | El overlay se comía el clic — **nunca** fue clicable |
| «El canal dejó de emitir» | El canal emitía; nos caímos de su ventana en vivo |
| «Tu navegador no puede reproducir este formato. Prueba en Safari» | El navegador podía; el origen servía relleno. Safari habría fallado igual |

Y dos Critical llegaron hasta el review final de rama: el dedup tiraba las
cabeceras de 753 de 982 streams, y `streamColumns` no seleccionaba esas columnas,
dejando una tarea entera en código muerto. **Ninguno era cazable con pruebas
unitarias**, porque los dobles rellenan campos que el repositorio real no
rellenaba.

El patrón es siempre el mismo: **la maquinaria funcionaba; lo que mentía era
todo lo que contaba sobre sí misma.** Las pruebas afirman que el código hace lo
que dice — no pueden decirte que lo que dice es falso.

**Fuera de alcance:** cualquier cambio de comportamiento del reproductor. Este
spec solo añade verificación. Si al escribirla aparece un bug, se arregla en su
propia rama, no aquí.

---

## 2. Lo que YA hay (y por qué esto es extender, no construir)

Descubierto al explorar, y cambia el tamaño del trabajo a la baja:

- **Playwright ya corre en CI** (`.github/workflows/ci.yml:148-161`) sobre
  **chromium, firefox y webkit**, con subida del informe si falla.
- Ya hay specs e2e: `catalogo`, `escenario`, `guia`, `reproduccion`.
- Ya hay un **servidor de fixtures** (`web/tests/e2e/servidor-fixtures.ts`) que
  sirve un M3U y HLS real (`canal.m3u8` + 5 segmentos `.ts`), y que ya sabe
  devolver 404.
- `reproduccion.spec.ts` ya cubre dos casos: un canal reproduce por el proxy con
  la posición avanzando, y el guard avisa cuando el stream muere.

**Lo que falta no es infraestructura: son los orígenes HOSTILES.** Los cuatro
fallos de arriba vienen de orígenes que se portan mal de formas concretas, y el
servidor de fixtures hoy solo sabe portarse bien o devolver 404.

---

## 3. Arquitectura: dos piezas con contratos distintos

La distinción que hace que esto funcione, y que hay que respetar:

> **Un gate tiene que ser determinista. Una medición puede ser ruidosa.**

Mezclarlas es lo que convierte un e2e en algo que se ignora porque «falla a
veces».

### 3.1 Parte A — Fixtures adversarios (hermético, en CADA commit, es un GATE)

Se amplía `servidor-fixtures.ts` con orígenes que reproducen, byte a byte, los
fallos reales medidos ayer:

| Origen falso | Qué hace | Reproduce |
|---|---|---|
| `/hostil/relleno` | HTTP 200, `video/MP2T`, **188 bytes** (un paquete TS nulo) | AXN: segmento caducado disfrazado de respuesta válida |
| `/hostil/lento` | manifiesto instantáneo, segmentos con retardo configurable | AMC: vivo para el health-check, imposible de sostener |
| `/hostil/403` | 403 en el manifiesto | geo-bloqueo / hotlink |
| `/hostil/404` | 404 en el manifiesto | mirror caducado (ya existe, se formaliza) |
| `/hostil/rota` | arranca bien y a mitad pasa a servir relleno | el corte de AXN en vivo |

Sobre esos orígenes, las aserciones son **sobre lo que el usuario lee**, no sobre
estado interno:

1. Relleno → dice «la señal llega rota», **NO** «tu navegador no puede con el
   formato», y **no** menciona Safari.
2. Lento → no se anuncia como «caducó».
3. Dos mirrors con causas distintas → mensaje genérico + «Se probaron 2 mirrors».
4. **La tarjeta de error es utilizable**: el botón se puede pulsar de verdad
   (`click()` real de Playwright, no `dispatchEvent`) y relanza un intento.
5. `document.visibilityState === 'hidden'` → **no** aparece tarjeta de error; se
   queda en «Conectando…» y arranca al volver a verse.
6. Un no-fatal transitorio antes de arrancar no mata el intento.

**Criterio de aceptación, y es el que importa:** cada prueba se valida
**revirtiendo su arreglo** y comprobando que la prueba se pone roja. Una prueba
que no se ha visto fallar contra el bug que dice cubrir no cuenta. Los cuatro
bugs de la tabla del §1 tienen que quedar cubiertos así.

### 3.2 Parte B — Censo nocturno contra el catálogo real (NO es un gate)

Un job aparte, `nightly`, que abre el catálogo de verdad y mide:

- reproduce N canales (arranca en 50) elegidos de forma estable (hash del id, no
  aleatorio, para poder comparar entre noches);
- por cada uno: ¿llega a `readyState 4` con la posición avanzando en 20 s?
- publica **tasa reproducible**, p50/p90 de tiempo hasta el primer fotograma, y
  el reparto de clases de fallo.

**Nunca falla el build.** Los orígenes de terceros cambian solos; convertir eso
en un gate es enseñar al equipo a ignorar los rojos. Su salida es un artefacto
con historial, y la señal que se vigila es la **tendencia**: si la tasa
reproducible cae 10 puntos de una noche a otra, eso sí merece una alerta.

Esto también da, por fin, el número honesto que hoy falta: cuántos canales se
ven **de verdad**, frente a los 9.304 que el health-check llama vivos. La
estimación actual (~6.500) sale de extrapolar una muestra de 30 y no vale como
métrica de producto.

---

## 4. Lo que este spec NO promete

- **No caza todo.** El fallo de la pestaña oculta depende de que el navegador
  reporte `hidden`; en headless hay que forzarlo con CDP
  (`Emulation.setPageVisibilityOverride` o equivalente). Si no se puede en los
  tres navegadores, se cubre en chromium y se dice en el propio test.
- **No sustituye a mirar la app.** Sigue siendo obligatorio abrirla al cambiar
  CSS: las pruebas no ven maquetación (regla ya escrita en `CLAUDE.md`). Lo que
  sí hace es que el fallo de apilamiento tenga una prueba que lo fija una vez
  cazado.
- **No mide calidad de imagen.** `readyState 4` y posición avanzando dicen «se
  ve algo», no «se ve bien».

---

## 5. Riesgos

- **Duración del e2e.** Ya corre en tres navegadores; añadir seis orígenes
  hostiles lo alarga. Mitigación: los tests hostiles solo en chromium
  (`test.describe.configure`), que es donde vive el 100% de los bugs de ayer;
  los tres navegadores se reservan para las rutas felices que ya cubren.
- **El censo nocturno consume red y tiempo.** 50 canales × 20 s con
  concurrencia 5 son ~4 minutos. Aceptable para un nightly.
- **Falsos verdes por fixture demasiado amable.** El `/hostil/lento` tiene que
  ser lento de verdad respecto al presupuesto del guard (7 s sin progreso), no
  «un poco lento».

---

## 6. Qué NO cambia

El reproductor, el guard, el proxy, el syncer, `mobile/`, y el contrato de 15
claves. Este spec añade `web/tests/e2e/` y un workflow; nada de producción.
