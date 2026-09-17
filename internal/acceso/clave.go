// Package acceso protege Open TV cuando escucha fuera de loopback: una clave
// por instalación, sesiones firmadas sin estado y una página de acceso sin
// JavaScript. En loopback no se monta nada de esto.
package acceso

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FicheroClave vive junto a la base de datos, en el mismo directorio de datos.
const FicheroClave = "access-key"

// CargarOGenerar decide la clave de acceso. La variable de entorno gana
// siempre (es lo cómodo en Docker); si no hay, se reutiliza la del fichero; y
// si tampoco, se genera una y se guarda con permisos 0600. generada dice si
// hay que enseñársela a quien arranca, porque no la conoce nadie más.
func CargarOGenerar(dir, deEntorno string) (string, bool, error) {
	if c := strings.TrimSpace(deEntorno); c != "" {
		return c, false, nil
	}
	ruta := filepath.Join(dir, FicheroClave)
	datos, err := os.ReadFile(ruta) //nolint:gosec // ruta = directorio de datos propio + nombre fijo
	if err == nil {
		c := strings.TrimSpace(string(datos))
		if c == "" {
			return "", false, fmt.Errorf("acceso: %s está vacío; bórralo para generar una clave nueva", ruta)
		}
		return c, false, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", false, fmt.Errorf("acceso: leyendo %s: %w", ruta, err)
	}

	crudo := make([]byte, 32)
	if _, err := rand.Read(crudo); err != nil {
		return "", false, fmt.Errorf("acceso: generando la clave: %w", err)
	}
	c := base64.RawURLEncoding.EncodeToString(crudo)
	if err := os.WriteFile(ruta, []byte(c+"\n"), 0o600); err != nil {
		return "", false, fmt.Errorf("acceso: guardando %s: %w", ruta, err)
	}
	return c, true, nil
}
