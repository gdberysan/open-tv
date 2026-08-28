## Qué cambia y por qué

<!-- El porqué importa más que el qué: el diff ya cuenta el qué. -->

## Issue relacionado

<!-- Si es más que un arreglo pequeño, debería haber uno. Ver CONTRIBUTING.md. -->

## Comprobado

- [ ] `gofmt -l .` sin salida · `go vet ./...` · `go build ./...`
- [ ] `go test -race -count=1 ./...`
- [ ] `golangci-lint run ./...` (recuerda: `//nolint:gosec`, no `//nosec`)
- [ ] `go run ./tools/scrubcheck`
- [ ] `cd web && npm run check && npm test && npm run build`
- [ ] `internal/ui/dist/.gitkeep` sigue ahí (el build del cliente lo borra)
- [ ] `mobile/` sin tocar

## Si cambia algo que se ve o se oye

<!-- Capturas, o cómo mirarlo. Las pruebas no ven maquetación: un CSS puede
     pasar todos los gates y salir cortado en un teléfono. -->
