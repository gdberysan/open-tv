// Package proxy relaya HLS para el navegador de la MISMA máquina.
//
// Existe porque Chrome y Firefox necesitan CORS y el 32 % de los streams vivos
// del catálogo no lo mandan (censo 2026-08-22). Revierte, de forma deliberada
// y acotada, la regla "el gateway nunca proxya vídeo": es el precio de esos
// dos navegadores. Solo se monta si la dirección de escucha es loopback, y
// nunca forma parte del sitio hospedado.
package proxy

import (
	"net/url"
	"strings"
)

// ReescribirManifiesto reescribe TODAS las URIs de un manifiesto HLS para que
// pasen por prefijo (p.ej. "/proxy/hls?u=").
//
// Reescribir solo los segmentos no sirve: el navegador pediría la clave, el
// mapa de inicialización y las pistas de audio directamente al origen, y
// volvería a chocar con el mismo CORS por el que existe el proxy.
//
// base es la URL absoluta del manifiesto, necesaria para resolver las
// referencias relativas.
func ReescribirManifiesto(base *url.URL, cuerpo, prefijo string) string {
	lineas := strings.Split(cuerpo, "\n")
	for i, linea := range lineas {
		recortada := strings.TrimSpace(linea)
		switch {
		case recortada == "":
			// Se deja como está, con su \r si lo tenía.
		case strings.HasPrefix(recortada, "#"):
			lineas[i] = reescribirAtributoURI(base, linea, prefijo)
		default:
			lineas[i] = envolver(base, recortada, prefijo)
		}
	}
	return strings.Join(lineas, "\n")
}

// reescribirAtributoURI cambia el valor de URI="..." en una línea de etiqueta.
// Genérico a propósito: vale para EXT-X-KEY, EXT-X-MAP, EXT-X-MEDIA,
// EXT-X-I-FRAME-STREAM-INF, EXT-X-PART, EXT-X-PRELOAD-HINT y cualquier
// etiqueta futura que use el mismo atributo. Una lista de etiquetas conocidas
// se quedaría corta en silencio.
func reescribirAtributoURI(base *url.URL, linea, prefijo string) string {
	const marca = `URI="`
	i := strings.Index(linea, marca)
	if i < 0 {
		return linea
	}
	inicio := i + len(marca)
	fin := strings.Index(linea[inicio:], `"`)
	if fin < 0 {
		return linea
	}
	valor := linea[inicio : inicio+fin]
	return linea[:inicio] + envolver(base, valor, prefijo) + linea[inicio+fin:]
}

// envolver resuelve ref contra base y la mete en el prefijo del proxy. Si la
// referencia no se puede interpretar se devuelve tal cual: un manifiesto con
// una línea rara reproduce el resto; uno al que le hemos comido una línea, no.
func envolver(base *url.URL, ref, prefijo string) string {
	abs, err := base.Parse(ref)
	if err != nil {
		return ref
	}
	return prefijo + url.QueryEscape(abs.String())
}
