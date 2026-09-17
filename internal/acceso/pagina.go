package acceso

import (
	"html/template"
	"net/http"
	"strings"
)

type textos struct {
	Lang, Titulo, Explicacion, Etiqueta, Boton, Error, Ayuda string
}

var textosPorIdioma = map[string]textos{
	"es": {
		Lang:        "es",
		Titulo:      "Korven Open TV",
		Explicacion: "Este Open TV es accesible desde la red, así que pide la clave de acceso.",
		Etiqueta:    "Clave de acceso",
		Boton:       "Entrar",
		Error:       "La clave no es correcta.",
		Ayuda:       "La clave aparece en el registro del servidor al arrancar por primera vez, o con: open-tv access-key",
	},
	"en": {
		Lang:        "en",
		Titulo:      "Korven Open TV",
		Explicacion: "This Open TV is reachable from the network, so it asks for the access key.",
		Etiqueta:    "Access key",
		Boton:       "Sign in",
		Error:       "That key isn't correct.",
		Ayuda:       "The key is printed in the server log on first start, or run: open-tv access-key",
	},
}

// idiomaDe elige es o en por Accept-Language. Español por defecto, como el
// resto de la app.
func idiomaDe(r *http.Request) textos {
	for _, parte := range strings.Split(r.Header.Get("Accept-Language"), ",") {
		etiqueta := strings.ToLower(strings.TrimSpace(strings.SplitN(parte, ";", 2)[0]))
		if strings.HasPrefix(etiqueta, "es") {
			return textosPorIdioma["es"]
		}
		if strings.HasPrefix(etiqueta, "en") {
			return textosPorIdioma["en"]
		}
	}
	return textosPorIdioma["es"]
}

// Los colores son los tokens de marca de web/src/estilos/tokens/colors.css
// copiados a mano: esta página se sirve ANTES de la sesión, sin acceso a los
// assets del cliente.
var plantilla = template.Must(template.New("acceso").Parse(`<!doctype html>
<html lang="{{.T.Lang}}">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.T.Titulo}}</title>
<style>
:root{color-scheme:dark;--grafito:#0E131B;--carbon:#171E29;--ambar:#FF8A2B;--acero:#97A3B2;--hueso:#EFF3F8;--error:#E5604D}
*{box-sizing:border-box}
body{margin:0;min-height:100vh;display:grid;place-items:center;background:var(--grafito);color:var(--hueso);font:16px/1.5 system-ui,-apple-system,"Segoe UI",sans-serif;padding:16px}
main{width:100%;max-width:380px;background:var(--carbon);border:1px solid #283142;border-radius:16px;padding:28px}
h1{margin:0 0 8px;font-size:22px}h1 span{color:var(--ambar)}
p{margin:0 0 20px;color:var(--acero)}
label{display:block;font-weight:600;margin-bottom:6px}
input{width:100%;padding:10px 12px;border-radius:10px;border:1px solid #3a4658;background:var(--grafito);color:var(--hueso);font:inherit}
input:focus-visible,button:focus-visible{outline:3px solid var(--ambar);outline-offset:2px}
button{margin-top:16px;width:100%;padding:10px;border:0;border-radius:10px;background:var(--ambar);color:var(--grafito);font:inherit;font-weight:700;cursor:pointer}
.error{color:var(--error);margin:12px 0 0}
small{display:block;margin-top:20px;color:var(--acero)}
</style>
</head>
<body>
<main>
<h1>KORVEN <span>OPEN TV</span></h1>
<p>{{.T.Explicacion}}</p>
<form method="post" action="/acceso">
<label for="clave">{{.T.Etiqueta}}</label>
<input id="clave" name="clave" type="password" autocomplete="current-password" required autofocus{{if .ConError}} aria-invalid="true" aria-describedby="error"{{end}}>
{{if .ConError}}<p id="error" class="error" role="alert">{{.T.Error}}</p>{{end}}
<button type="submit">{{.T.Boton}}</button>
</form>
<small>{{.T.Ayuda}}</small>
</main>
</body>
</html>
`))

const cspPagina = "default-src 'none'; style-src 'unsafe-inline'; form-action 'self'; frame-ancestors 'none'; base-uri 'none'"

func pintar(w http.ResponseWriter, r *http.Request, estado int, conError bool) {
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Content-Security-Policy", cspPagina)
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("X-Frame-Options", "DENY")
	h.Set("Cache-Control", "no-store")
	w.WriteHeader(estado)
	_ = plantilla.Execute(w, struct {
		T        textos
		ConError bool
	}{idiomaDe(r), conError})
}
