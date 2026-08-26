package main

import (
	"fmt"
	"os/exec"
	"runtime"
)

// AbrirNavegador abre url en el navegador por defecto del sistema.
//
// No se usa una dependencia para esto: son tres comandos y ninguna librería
// va a saber más que el propio sistema. El error se registra pero nunca es
// fatal — el servidor ya está escuchando y la URL se imprime igualmente.
func AbrirNavegador(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url) // #nosec G204,G702 -- url sale de listenAddr (LISTEN_ADDR o el socket ya vinculado), config local, no de red; exec.Command no invoca shell
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url) // #nosec G204,G702 -- url sale de listenAddr (LISTEN_ADDR o el socket ya vinculado), config local, no de red; exec.Command no invoca shell
	default:
		cmd = exec.Command("xdg-open", url) // #nosec G204,G702 -- url sale de listenAddr (LISTEN_ADDR o el socket ya vinculado), config local, no de red; exec.Command no invoca shell
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("abriendo el navegador: %w", err)
	}
	// Release y no Wait: `open` en macOS termina enseguida, pero xdg-open puede
	// quedarse vivo mientras dure el navegador. Esperarlo colgaría el arranque.
	return cmd.Process.Release()
}
