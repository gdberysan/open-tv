# Política de seguridad

> English version below.

## Cómo reportar

**No abras un issue público** para un fallo de seguridad.

1. **Preferido:** el reporte privado de GitHub — pestaña *Security* →
   *Report a vulnerability*. Llega solo a quien mantiene el repo.
2. **Alternativa:** `gdberysan@gmail.com`, con `[open-tv]` en el asunto.

Respuesta en **72 horas** para confirmar que se recibió, y una evaluación
inicial en **7 días**. Esto lo mantiene una sola persona y no hay programa de
recompensas: lo que sí hay es crédito en las notas de la versión que lo
corrija, salvo que prefieras lo contrario.

## Qué versiones se atienden

La **última release** publicada. No se hacen parches retroactivos a versiones
anteriores.

## Modelo de amenaza, en corto

Open TV es **un solo binario que corre en la máquina de quien lo usa**. No hay
cuentas, ni telemetría, ni servidor nuestro en medio: el catálogo lo aporta la
persona y los flujos van del navegador a cada emisora. Eso deja una superficie
pequeña y muy concreta:

- El servidor HTTP local y su interfaz web embebida.
- El **proxy HLS**, que solo relaya URLs del catálogo o firmadas por el propio
  proceso (HMAC), con guarda anti-SSRF (control de conexión, verificación en
  cada redirección y tope de tamaño).
- El **modo red**: fuera de loopback, todo salvo `/health` y `/acceso` exige
  una sesión firmada que se obtiene con la clave de acceso de la instalación.
- El middleware de mismo origen (Host + `Sec-Fetch-Site` en métodos mutantes),
  contra CSRF y DNS-rebinding.
- La CSP estricta del cliente (`script-src 'self'`).
- El parseo de listas **M3U** y de guías **EPG/XMLTV**, que es entrada no
  confiable por definición: la aporta quien usa el programa.

## Dentro del alcance

- Escapar del proxy hacia direcciones que no debería alcanzar (SSRF), por
  redirección o por resolución DNS.
- Que algo accesible desde otra pestaña o desde la red local llegue a la API
  local.
- Saltarse la clave de acceso o la firma de sesión del modo red, o usar el
  proxy para relayar URLs que no están en el catálogo.
- Ejecución de código o escritura de ficheros fuera del directorio de datos a
  partir de una lista M3U o una guía EPG manipuladas.
- XSS en el cliente embebido, o cualquier forma de saltarse la CSP.
- Fuga de datos de la máquina hacia fuera: el programa no debe enviar nada.

## Fuera del alcance

- El **contenido** de los flujos de terceros y lo que haga cada emisora: aquí
  no se aloja ni se retransmite nada.
- Canales caídos, geobloqueados o con URLs caducadas. Eso es mantenimiento de
  catálogo, no seguridad.
- Ataques que requieran ya tener acceso físico o de administración a la
  máquina.
- Volumen de peticiones contra emisoras ajenas.
- Informes generados por escáneres sin una explicación de por qué eso es
  explotable aquí.

---

# Security policy

## Reporting a vulnerability

**Please don't open a public issue** for a security problem.

1. **Preferred:** GitHub private reporting — *Security* tab → *Report a
   vulnerability*. It reaches the maintainer only.
2. **Alternative:** `gdberysan@gmail.com`, with `[open-tv]` in the subject.

You'll get an acknowledgement within **72 hours** and an initial assessment
within **7 days**. This is maintained by one person and there's no bounty
programme — there is credit in the release notes that fix it, unless you'd
rather not be named.

## Supported versions

The **latest release**. Earlier versions don't get backported patches.

## Threat model, briefly

Open TV is **a single binary running on the user's own machine**. No accounts,
no telemetry, no server of ours in between: the user supplies the catalogue and
streams go from their browser straight to each broadcaster. In scope:

- The local HTTP server and its embedded web client.
- The **HLS proxy**, which only relays catalog URLs or URLs signed by the
  process itself (HMAC), with SSRF guards (connection control, per-redirect
  checks, size cap).
- **Network mode**: outside loopback, everything except `/health` and
  `/acceso` requires a signed session obtained with the installation's access
  key.
- The same-origin middleware (Host + `Sec-Fetch-Site` on mutating methods),
  against CSRF and DNS rebinding.
- The client's strict CSP (`script-src 'self'`).
- Parsing of **M3U** playlists and **EPG/XMLTV** guides — untrusted input by
  definition, since the user supplies it.

## Out of scope

- The **content** of third-party streams and what each broadcaster does: nothing
  is hosted or re-transmitted here.
- Dead, geo-blocked, or expired channels. That's catalogue maintenance.
- Attacks requiring existing physical or administrative access to the machine.
- Request volume against third-party broadcasters.
- Scanner output without an explanation of why it's exploitable here.
