// Package datadir resuelve dónde vive la base de datos del catálogo.
//
// Existe porque DB_PATH es relativo al directorio de trabajo: ejecutar
// `open-tv` desde dos sitios distintos son dos catálogos distintos, y
// ejecutarlo desde / crea una DB vacía que responde 200 con cero canales.
// Para un binario que un desconocido lanza desde donde sea, eso no vale.
package datadir

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// nombreApp es el del directorio en macOS y Windows, donde la convención es
// el nombre legible del producto. En Linux la convención es en minúsculas
// y con guiones.
const (
	nombreApp     = "Korven Open TV"
	nombreUnix    = "korven-open-tv"
	nombreFichero = "iptv.db"
)

// Default devuelve el directorio de datos del sistema. No crea nada.
func Default() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("datadir.Default: %w", err)
		}
		return filepath.Join(home, "Library", "Application Support", nombreApp), nil

	case "windows":
		if appdata := os.Getenv("APPDATA"); appdata != "" {
			return filepath.Join(appdata, nombreApp), nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("datadir.Default: %w", err)
		}
		return filepath.Join(home, "AppData", "Roaming", nombreApp), nil

	default:
		if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
			return filepath.Join(xdg, nombreUnix), nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("datadir.Default: %w", err)
		}
		return filepath.Join(home, ".local", "share", nombreUnix), nil
	}
}

// RutaDB devuelve la ruta del fichero SQLite y deja su directorio creado.
// DB_PATH manda si está puesta: es la vía de escape documentada.
func RutaDB() (string, error) {
	if p := os.Getenv("DB_PATH"); p != "" {
		return p, nil
	}
	dir, err := Default()
	if err != nil {
		return "", err
	}
	// 0o700 y no 0o755: aquí no hay secretos, pero tampoco hay motivo para que
	// otro usuario de la máquina lea qué canales ves.
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("datadir.RutaDB (MkdirAll %s): %w", dir, err)
	}
	return filepath.Join(dir, nombreFichero), nil
}
