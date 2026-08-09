package domain

import "strings"

// AirplaySupport clasifica si un stream puede enviarse a un receptor AirPlay.
//
// Tres estados y no dos: solo el 57,5 % de los manifiestos del catálogo declara
// CODECS (medido sobre 40 streams vivos), así que "no se sabe" es el caso más
// común después de "sí". Colapsarlo en "no" marcaría como rotos miles de
// canales que funcionan.
type AirplaySupport int

const (
	AirplayUnknown AirplaySupport = iota
	AirplayNo
	AirplayOK
)

// codecsSoportados son los prefijos de RFC 6381 que AVFoundation reproduce.
// Lista de permitidos, no de prohibidos: un códec desconocido no se asume
// reproducible.
var codecsSoportados = []string{
	"avc1.", "avc3.", "hvc1.", "hev1.", "dvh1.", "dvhe.",
	"mp4a.40.", "ac-3", "ec-3", "alac",
}

// keyformatApple es el único sistema de claves que AVFoundation abre por sí
// solo. Widevine y PlayReady sobre SAMPLE-AES no viajan a AirPlay.
const keyformatApple = "com.apple.streamingkeydelivery"

// ClassifyManifest decide si un stream es reproducible por AirPlay a partir de
// su URL y del cuerpo de su manifiesto. No hace E/S: el cuerpo llega ya leído.
func ClassifyManifest(url, body string) AirplaySupport {
	if urlNoReproducible(url) {
		return AirplayNo
	}
	if cifradoNoApple(body) {
		return AirplayNo
	}

	hayVarianteSinDeclarar := false
	hayVarianteDeclarada := false

	for _, linea := range strings.Split(body, "\n") {
		linea = strings.TrimSpace(linea)
		if !strings.HasPrefix(linea, "#EXT-X-STREAM-INF:") {
			continue
		}
		codecs, ok := atributoEntreComillas(linea, "CODECS")
		if !ok {
			hayVarianteSinDeclarar = true
			continue
		}
		hayVarianteDeclarada = true
		if varianteSoportada(codecs) {
			return AirplayOK
		}
	}

	// Ninguna variante declarada sirve, pero alguna no se declaró: no hay base
	// para afirmar que el canal es incompatible.
	if hayVarianteDeclarada && !hayVarianteSinDeclarar {
		return AirplayNo
	}
	return AirplayUnknown
}

func urlNoReproducible(url string) bool {
	u := strings.ToLower(url)
	return strings.Contains(u, ".mpd") ||
		strings.HasPrefix(u, "rtmp://") ||
		strings.HasPrefix(u, "rtmps://") ||
		strings.HasPrefix(u, "mmsh://") ||
		strings.HasPrefix(u, "mms://")
}

// cifradoNoApple detecta SAMPLE-AES con un sistema de claves que no es el de
// Apple. Sin KEYFORMAT no se concluye nada: el default es de Apple.
func cifradoNoApple(body string) bool {
	for _, linea := range strings.Split(body, "\n") {
		linea = strings.TrimSpace(linea)
		if !strings.HasPrefix(linea, "#EXT-X-KEY:") {
			continue
		}
		if !strings.Contains(linea, "METHOD=SAMPLE-AES") {
			continue
		}
		formato, ok := atributoEntreComillas(linea, "KEYFORMAT")
		if ok && !strings.EqualFold(formato, keyformatApple) {
			return true
		}
	}
	return false
}

// varianteSoportada exige que TODOS los códecs de la variante sean
// reproducibles: un vídeo válido con un audio que AVFoundation no abre deja la
// variante inservible igualmente.
func varianteSoportada(codecs string) bool {
	partes := strings.Split(codecs, ",")
	if len(partes) == 0 {
		return false
	}
	for _, c := range partes {
		if !codecSoportado(strings.TrimSpace(c)) {
			return false
		}
	}
	return true
}

func codecSoportado(c string) bool {
	for _, p := range codecsSoportados {
		if strings.HasPrefix(c, p) {
			return true
		}
	}
	return false
}

// atributoEntreComillas extrae CLAVE="valor" de una línea de atributos HLS.
func atributoEntreComillas(linea, clave string) (string, bool) {
	i := strings.Index(linea, clave+`="`)
	if i < 0 {
		return "", false
	}
	resto := linea[i+len(clave)+2:]
	fin := strings.Index(resto, `"`)
	if fin < 0 {
		return "", false
	}
	return resto[:fin], true
}
