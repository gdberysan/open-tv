# Cómo contribuir

> English version below.

Gracias por mirar el código. Esto lo mantiene una sola persona, así que
conviene decir de entrada qué encaja y qué no.

## Antes de escribir código

**Abre un issue primero** si el cambio es más que un arreglo pequeño. No por
burocracia: hay decisiones de diseño que no se ven desde fuera y sería una
lástima que gastaras una tarde en algo que se va a rechazar por un motivo que
no estaba escrito en ninguna parte.

Los arreglos evidentes —un typo, un error claro, un caso límite— pueden llegar
directos como pull request.

## Lo que este proyecto NO va a aceptar

Vale la pena ser explícito para no hacerte perder el tiempo:

- **Catálogos de canales de fábrica.** Una instalación limpia arranca vacía a
  propósito: quien usa el programa decide qué añade. No es una carencia.
- **Cuentas, registro o telemetría**, ni "anónima". Nada sale de la máquina
  (el modo red usa una única clave de acceso por instalación, no cuentas).
- **Servicios en la nube** de por medio. El reproductor habla directo con cada
  emisora; el proxy local existe solo para los casos en que el navegador no
  puede con el flujo, y **solo relaya URLs del catálogo**.
- **Saltarse protecciones**: DRM, cabeceras falsificadas, flujos cifrados o
  cualquier cosa que sirva para acceder a lo que no está en abierto.
- **Cambios en `mobile/`.** La app de Flutter está congelada y su contrato JSON
  también. Si el backend necesita algo nuevo, se hace con tablas y endpoints
  nuevos, nunca alterando los que ya existen.

## El listón técnico

Todo esto corre en CI y bloquea la fusión:

```sh
gofmt -l .            # sin salida
go vet ./...
go build ./...
go test -race -count=1 ./...
golangci-lint run ./...
go run ./tools/scrubcheck

cd web && npm run check && npm test && npm run build
```

Un par de cosas que no son obvias:

- **`golangci-lint` no honra los `//nosec` de gosec**: si necesitas silenciar
  algo, es `//nolint:gosec` y con el motivo escrito al lado.
- El cliente web se construye a `internal/ui/dist` y el binario lo embebe con
  `go:embed`. El build **borra `internal/ui/dist/.gitkeep`**; hay que
  restaurarlo antes de commitear.
- Las pruebas de Go llevan `-race` siempre. Un fallo intermitente ahí es un
  fallo, no mala suerte.

## Estilo

- **Español** en el código, los comentarios y los mensajes de commit. La
  interfaz va en español e inglés, a la par.
- Los comentarios explican **por qué**, no qué. Si hace falta contar qué hace
  una línea, casi siempre es mejor reescribir la línea.
- Commits en formato Conventional Commits (`feat:`, `fix:`, `docs:`…), con el
  cuerpo explicando la razón del cambio.
- Nada de dependencias nuevas sin una razón de peso, ni en Go ni en JS. El
  bundle propio del cliente tiene un tope de **80 KB gzip**.

## Seguridad

Los fallos de seguridad **no** van en un issue público. Están en
[`SECURITY.md`](SECURITY.md) las dos vías privadas.

## Licencia

El código va con licencia MIT. La **marca** (el nombre «Korven», el wordmark y
el emblema) tiene su propia licencia y no entra en ella: si publicas un fork,
ponle tu nombre.

---

# Contributing

Thanks for looking at the code. This is maintained by one person, so it's worth
saying up front what fits and what doesn't.

## Before writing code

**Open an issue first** for anything beyond a small fix. Not bureaucracy: some
design decisions aren't visible from outside, and it would be a shame to spend
an afternoon on something that gets turned down for a reason nobody wrote down.

Obvious fixes — a typo, a clear bug, an edge case — can come straight as a PR.

## What this project will NOT take

- **Bundled channel catalogues.** A clean install starts empty on purpose.
- **Accounts, sign-up or telemetry**, not even "anonymous" kinds (network mode
  uses a single access key per installation, not accounts).
- **Cloud services** in the path. The player talks straight to each broadcaster;
  the local proxy exists only for streams the browser can't handle on its own,
  and it **only relays catalog URLs**.
- **Circumvention**: DRM, spoofed headers, encrypted streams, or anything meant
  to reach what isn't broadcast in the open.
- **Changes under `mobile/`.** The Flutter app is frozen, and so is its JSON
  contract. Backend changes go in new tables and new endpoints, never by
  altering the existing ones.

## The technical bar

All of this runs in CI and blocks merging — see the commands above.

Two non-obvious things: **`golangci-lint` doesn't honour gosec's `//nosec`**
(use `//nolint:gosec`, with the reason next to it), and the web build **deletes
`internal/ui/dist/.gitkeep`**, which has to be restored before committing.

## Style

Spanish in code, comments and commit messages; the UI ships Spanish and English
at parity. Comments explain **why**, not what. Conventional Commits. No new
dependencies without a strong reason, and the client's own bundle stays under
**80 KB gzip**.

## Security

Security bugs don't go in public issues — see [`SECURITY.md`](SECURITY.md).

## Licence

Code is MIT. The **brand** (the "Korven" name, wordmark and emblem) is licensed
separately and isn't included: if you publish a fork, give it your own name.
