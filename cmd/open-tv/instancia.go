package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"
)

// InstanciaViva dice si en base hay OTRO Open TV escuchando.
//
// Se comprueba antes de rendirse por "puerto ocupado": lo más probable cuando
// el 8080 está cogido es que el usuario ya tenga Open TV abierto, y entonces
// lo correcto es llevarle a esa pestaña, no arrancar un segundo catálogo.
// La marca es web_ui: solo la pone este binario.
func InstanciaViva(ctx context.Context, base string) bool {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/health", nil) // #nosec G704 -- base sale de listenAddr (LISTEN_ADDR o 127.0.0.1:8080 por defecto), config local, no de red
	if err != nil {
		return false
	}
	resp, err := http.DefaultClient.Do(req) // #nosec G704 -- misma petición local de arriba
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return false
	}

	var cuerpo struct {
		WebUI *bool `json:"web_ui"`
	}
	datos, err := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
	if err != nil {
		return false
	}
	if err := json.Unmarshal(datos, &cuerpo); err != nil {
		return false
	}
	return cuerpo.WebUI != nil
}
