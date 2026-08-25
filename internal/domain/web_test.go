package domain_test

import (
	"testing"

	"github.com/gdberysan/open-tv/internal/domain"
)

const webMasterOK = `#EXTM3U
#EXT-X-STREAM-INF:BANDWIDTH=1200000,CODECS="avc1.4d401f,mp4a.40.2"
chunk.m3u8
`

const webMasterHEVC = `#EXTM3U
#EXT-X-STREAM-INF:BANDWIDTH=6000000,CODECS="hvc1.1.6.L93.B0,mp4a.40.2"
chunk.m3u8
`

const webMasterSinCodecs = `#EXTM3U
#EXT-X-STREAM-INF:BANDWIDTH=1200000
chunk.m3u8
`

func TestClassifyWeb(t *testing.T) {
	casos := []struct {
		nombre string
		url    string
		acao   string
		cuerpo string
		quiero domain.WebSupport
	}{
		{"https con ACAO comodín y códecs de navegador",
			"https://cdn.example/live.m3u8", "*", webMasterOK, domain.WebOK},

		{"https con ACAO exactamente nuestro origen",
			"https://cdn.example/live.m3u8", domain.OrigenWeb, webMasterOK, domain.WebOK},

		// El caso Pluto TV: 1 359 streams del catálogo mandan ACAO pero solo
		// para su propio dominio. Desde nuestra página el navegador los corta.
		{"ACAO de otro origen no sirve",
			"https://cdn.example/live.m3u8", "https://pluto.tv", webMasterOK, domain.WebNo},

		{"sin ACAO no hay fetch posible",
			"https://cdn.example/live.m3u8", "", webMasterOK, domain.WebNo},

		// El sitio hospedado es HTTPS: contenido mixto bloqueado. En el binario
		// local el proxy de loopback cubre justo este caso.
		{"http es contenido mixto",
			"http://cdn.example/live.m3u8", "*", webMasterOK, domain.WebNo},

		{"códecs que ningún navegador decodifica",
			"https://cdn.example/live.m3u8", "*", webMasterHEVC, domain.WebNo},

		// Solo el 57,5 % de los manifiestos declara CODECS. Sin declaración no
		// hay base para decir que no.
		{"sin CODECS declarados se acepta",
			"https://cdn.example/live.m3u8", "*", webMasterSinCodecs, domain.WebOK},

		{"cuerpo vacío: manda esquema y CORS",
			"https://cdn.example/canal.ts", "*", "", domain.WebOK},

		{"DASH no lo toca ningún navegador sin MSE propio",
			"https://cdn.example/manifest.mpd", "*", "", domain.WebNo},

		{"rtmp no existe en el navegador",
			"rtmp://cdn.example/live", "*", "", domain.WebNo},

		// hls.js abre AES-128 pero no FairPlay: SAMPLE-AES es no.
		{"SAMPLE-AES no se abre en el navegador",
			"https://cdn.example/live.m3u8", "*",
			"#EXTM3U\n#EXT-X-KEY:METHOD=SAMPLE-AES,URI=\"skd://x\"\n" + webMasterOK,
			domain.WebNo},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if got := domain.ClassifyWeb(c.url, c.acao, c.cuerpo); got != c.quiero {
				t.Errorf("ClassifyWeb(%q, %q, …) = %v, quiero %v", c.url, c.acao, got, c.quiero)
			}
		})
	}
}

// El cero valor tiene que ser Unknown: StreamResult.Web nace así cuando el
// checker no llegó a hablar con el origen, y eso es exactamente "no se sabe".
func TestWebSupportCeroEsUnknown(t *testing.T) {
	var v domain.WebSupport
	if v != domain.WebUnknown {
		t.Errorf("el cero valor es %v, quiero WebUnknown", v)
	}
}
