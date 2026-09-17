package proxy_test

import (
	"net/url"
	"strings"
	"testing"

	"github.com/gdberysan/open-tv/internal/proxy"
)

const prefijo = "/proxy/hls?u="

func base(t *testing.T) *url.URL {
	t.Helper()
	u, err := url.Parse("https://cdn.example/live/master.m3u8")
	if err != nil {
		t.Fatalf("base: %v", err)
	}
	return u
}

func TestReescribeVariantesRelativasYAbsolutas(t *testing.T) {
	entrada := strings.Join([]string{
		"#EXTM3U",
		"#EXT-X-STREAM-INF:BANDWIDTH=1200000,CODECS=\"avc1.4d401f\"",
		"720/chunk.m3u8",
		"#EXT-X-STREAM-INF:BANDWIDTH=400000",
		"https://otro.example/480/chunk.m3u8?t=9",
		"",
	}, "\n")

	got := proxy.ReescribirManifiesto(base(t), entrada, prefijo, nil)

	if !strings.Contains(got, prefijo+url.QueryEscape("https://cdn.example/live/720/chunk.m3u8")) {
		t.Errorf("la variante relativa no se resolvió contra la base:\n%s", got)
	}
	if !strings.Contains(got, prefijo+url.QueryEscape("https://otro.example/480/chunk.m3u8?t=9")) {
		t.Errorf("la variante absoluta no se envolvió:\n%s", got)
	}
	// Las líneas de etiqueta que no llevan URI se dejan intactas.
	if !strings.Contains(got, "#EXT-X-STREAM-INF:BANDWIDTH=1200000,CODECS=\"avc1.4d401f\"") {
		t.Errorf("se tocó una etiqueta sin URI:\n%s", got)
	}
}

// EXT-X-KEY y EXT-X-MAP llevan la URI en un atributo. Olvidarlos deja el
// stream cifrado sin clave y el fMP4 sin cabecera: silencio con manifiesto OK.
func TestReescribeAtributosURI(t *testing.T) {
	entrada := strings.Join([]string{
		"#EXTM3U",
		"#EXT-X-KEY:METHOD=AES-128,URI=\"clave.key\",IV=0x00",
		"#EXT-X-MAP:URI=\"init.mp4\"",
		"#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID=\"a\",URI=\"audio/es.m3u8\"",
		"seg1.ts",
		"",
	}, "\n")

	got := proxy.ReescribirManifiesto(base(t), entrada, prefijo, nil)

	for _, quiero := range []string{
		"https://cdn.example/live/clave.key",
		"https://cdn.example/live/init.mp4",
		"https://cdn.example/live/audio/es.m3u8",
		"https://cdn.example/live/seg1.ts",
	} {
		if !strings.Contains(got, prefijo+url.QueryEscape(quiero)) {
			t.Errorf("falta %q reescrito:\n%s", quiero, got)
		}
	}
	// El resto del atributo sobrevive: IV se pierde y el descifrado falla.
	if !strings.Contains(got, "IV=0x00") {
		t.Errorf("se perdió IV:\n%s", got)
	}
	if !strings.Contains(got, "METHOD=AES-128") {
		t.Errorf("se perdió METHOD:\n%s", got)
	}
}

// EXT-X-BYTERANGE se aplica a la URI de la línea siguiente, que sí se
// reescribe. La etiqueta en sí no se toca; el Range lo reenvía el handler.
func TestNoTocaByterangeNiComentarios(t *testing.T) {
	entrada := strings.Join([]string{
		"#EXTM3U",
		"#EXT-X-TARGETDURATION:6",
		"#EXTINF:6.0,",
		"#EXT-X-BYTERANGE:75232@0",
		"seg.ts",
		"#EXT-X-ENDLIST",
		"",
	}, "\n")

	got := proxy.ReescribirManifiesto(base(t), entrada, prefijo, nil)

	for _, intacta := range []string{"#EXT-X-TARGETDURATION:6", "#EXTINF:6.0,", "#EXT-X-BYTERANGE:75232@0", "#EXT-X-ENDLIST"} {
		if !strings.Contains(got, intacta) {
			t.Errorf("se tocó %q:\n%s", intacta, got)
		}
	}
	if !strings.Contains(got, prefijo+url.QueryEscape("https://cdn.example/live/seg.ts")) {
		t.Errorf("no se reescribió el segmento:\n%s", got)
	}
}

// Una URI que no se puede resolver se deja como estaba: un manifiesto con una
// línea rara sigue reproduciendo el resto; uno al que le hemos comido una
// línea, no.
func TestURIIlegibleSeDejaIntacta(t *testing.T) {
	entrada := "#EXTM3U\n://esto no es una URL\n"
	got := proxy.ReescribirManifiesto(base(t), entrada, prefijo, nil)
	if !strings.Contains(got, "://esto no es una URL") {
		t.Errorf("se perdió la línea ilegible:\n%s", got)
	}
}

func TestConservaElNumeroDeLineas(t *testing.T) {
	entrada := "#EXTM3U\n\nseg1.ts\nseg2.ts\n"
	got := proxy.ReescribirManifiesto(base(t), entrada, prefijo, nil)
	if a, b := strings.Count(entrada, "\n"), strings.Count(got, "\n"); a != b {
		t.Errorf("líneas: entrada %d, salida %d", a, b)
	}
}

func TestReescribirManifiestoFirmaCadaURL(t *testing.T) {
	f := firmadorDePrueba(t)
	entrada := "#EXTM3U\n#EXT-X-KEY:METHOD=AES-128,URI=\"clave.bin\"\n#EXTINF:6.0,\nseg1.ts\n"

	got := proxy.ReescribirManifiesto(base(t), entrada, prefijo, f.Firmar)

	for _, rel := range []string{"clave.bin", "seg1.ts"} {
		abs, err := base(t).Parse(rel)
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		quiero := prefijo + url.QueryEscape(abs.String()) + "&f=" + f.Firmar(abs.String())
		if !strings.Contains(got, quiero) {
			t.Errorf("falta %q en:\n%s", quiero, got)
		}
	}
}
