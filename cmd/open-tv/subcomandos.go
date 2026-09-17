package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/gdberysan/open-tv/internal/acceso"
	"github.com/gdberysan/open-tv/internal/datadir"
)

// mostrarClave imprime la clave de acceso del modo red. Solo lee: generar una
// clave desde aquí dejaría una que el servidor en marcha no conoce.
func mostrarClave(w io.Writer) int {
	if c := strings.TrimSpace(os.Getenv("OPEN_TV_ACCESS_KEY")); c != "" {
		fmt.Fprintln(w, c)
		return 0
	}
	dbPath, err := datadir.RutaDB()
	if err != nil {
		fmt.Fprintf(w, "no se pudo resolver el directorio de datos: %v\n", err)
		return 1
	}
	ruta := filepath.Join(filepath.Dir(dbPath), acceso.FicheroClave)
	datos, err := os.ReadFile(ruta) //nolint:gosec // ruta = directorio de datos propio + nombre fijo
	if errors.Is(err, os.ErrNotExist) {
		fmt.Fprintf(w, "todavía no hay clave: se crea al arrancar Open TV fuera de loopback (%s)\n", ruta)
		return 1
	}
	if err != nil {
		fmt.Fprintf(w, "no se pudo leer %s: %v\n", ruta, err)
		return 1
	}
	fmt.Fprintln(w, strings.TrimSpace(string(datos)))
	return 0
}

// healthcheck es el HEALTHCHECK de la imagen: distroless no trae curl.
func healthcheck(ctx context.Context, listenAddr string) int {
	if InstanciaViva(ctx, baseLocal(listenAddr)) {
		return 0
	}
	return 1
}

// baseLocal traduce LISTEN_ADDR a una URL de loopback con el mismo puerto: el
// healthcheck corre dentro del propio contenedor o máquina.
func baseLocal(listenAddr string) string {
	puerto := "8080"
	if listenAddr != "" {
		if _, p, err := net.SplitHostPort(listenAddr); err == nil && p != "" {
			puerto = p
		}
	}
	return "http://127.0.0.1:" + puerto
}
