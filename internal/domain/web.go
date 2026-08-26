package domain

import "strings"

// WebSupport clasifica si un stream se reproduce DIRECTAMENTE en un navegador,
// desde una página de otro origen servida por HTTPS.
//
// Hermano de AirplaySupport en semántica —tres estados, lista de permitidos,
// "no se sabe" es un veredicto legítimo— pero con reglas distintas: al
// navegador le importa el esquema y el CORS, cosas que a AVFoundation le dan
// igual porque no es una página web.
//
// Es el veredicto ESTRICTO, el de hls.js (Chrome, Firefox). Safari reproduce
// HLS de forma nativa sin necesitar CORS, así que reproduce más de lo que este
// veredicto admite; esa diferencia la resuelve el cliente al elegir motor, no
// este clasificador. Censo 2026-08-22: Safari 85 %, Chrome/Firefox 67 %.
type WebSupport int

const (
	WebUnknown WebSupport = iota
	WebNo
	WebOK
)

// OrigenWeb es el origen del sitio hospedado. El health-checker lo manda como
// cabecera Origin para que los servidores que reflejan el origen del
// solicitante contesten con un ACAO que podamos reconocer.
const OrigenWeb = "https://opentv.korven.dev"

// codecsWeb son los prefijos RFC 6381 que decodifica cualquier navegador
// moderno en HLS. Lista de permitidos: un códec desconocido no se asume
// reproducible. HEVC queda fuera a propósito — es el 0,9 % del catálogo y solo
// Safari lo abre.
var codecsWeb = []string{"avc1.", "avc3.", "mp4a.40."}

// ClassifyWeb decide sobre la URL FINAL (tras redirecciones), la cabecera
// Access-Control-Allow-Origin de esa respuesta y el cuerpo del manifiesto.
// El cuerpo puede venir vacío: entonces solo mandan esquema y CORS.
func ClassifyWeb(finalURL, acao, body string) WebSupport {
	if urlNoReproducible(finalURL) {
		return WebNo
	}
	// Contenido mixto: una página HTTPS no puede cargar medios por HTTP.
	if !strings.HasPrefix(strings.ToLower(finalURL), "https://") {
		return WebNo
	}
	if !corsPermisivo(acao) {
		return WebNo
	}
	if cifradoNoAbrible(body) {
		return WebNo
	}
	return veredictoCodecsWeb(body)
}

// corsPermisivo acepta el comodín o exactamente nuestro origen. Cualquier otro
// origen concreto —el caso Pluto TV, 1 359 streams— no nos sirve.
func corsPermisivo(acao string) bool {
	acao = strings.TrimSpace(acao)
	return acao == "*" || strings.EqualFold(acao, OrigenWeb)
}

// cifradoNoAbrible: hls.js descifra AES-128 por su cuenta, pero SAMPLE-AES es
// FairPlay y necesita EME con un servidor de licencias que no tenemos.
func cifradoNoAbrible(body string) bool {
	for _, linea := range strings.Split(body, "\n") {
		linea = strings.TrimSpace(linea)
		if strings.HasPrefix(linea, "#EXT-X-KEY:") && strings.Contains(linea, "METHOD=SAMPLE-AES") {
			return true
		}
	}
	return false
}

// veredictoCodecsWeb aplica la misma regla que AirPlay: basta una variante
// reproducible; si TODAS las declaradas son inservibles y ninguna se quedó sin
// declarar, es que no. Sin declaraciones, se acepta.
func veredictoCodecsWeb(body string) WebSupport {
	hayDeclarada := false
	haySinDeclarar := false

	for _, linea := range strings.Split(body, "\n") {
		linea = strings.TrimSpace(linea)
		if !strings.HasPrefix(linea, "#EXT-X-STREAM-INF:") {
			continue
		}
		codecs, ok := atributoEntreComillas(linea, "CODECS")
		if !ok {
			haySinDeclarar = true
			continue
		}
		hayDeclarada = true
		if varianteWebSoportada(codecs) {
			return WebOK
		}
	}

	if hayDeclarada && !haySinDeclarar {
		return WebNo
	}
	return WebOK
}

// varianteWebSoportada exige que TODOS los códecs de la variante sirvan: un
// vídeo H.264 con audio AC-3 deja la variante inservible en el navegador.
func varianteWebSoportada(codecs string) bool {
	partes := strings.Split(codecs, ",")
	if len(partes) == 0 {
		return false
	}
	for _, c := range partes {
		if !codecWebSoportado(strings.TrimSpace(c)) {
			return false
		}
	}
	return true
}

func codecWebSoportado(c string) bool {
	for _, p := range codecsWeb {
		if strings.HasPrefix(c, p) {
			return true
		}
	}
	return false
}
