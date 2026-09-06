package validator_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gdberysan/open-tv/internal/adapters/validator"
	"github.com/gdberysan/open-tv/internal/domain"
	"github.com/gdberysan/open-tv/internal/proxy"
)

// Paquetes TS sintéticos (misma construcción que en domain/mpegts_test.go;
// se repite aquí porque los helpers de test no se exportan entre paquetes).
func paqueteTS(pid int, seccion []byte) []byte {
	p := make([]byte, 188)
	for i := range p {
		p[i] = 0xff
	}
	p[0] = 0x47
	//nolint:gosec
	p[1] = 0x40 | byte(pid>>8)
	//nolint:gosec
	p[2] = byte(pid)
	p[3] = 0x10
	p[4] = 0
	copy(p[5:], seccion)
	return p
}

func seccionPSI(tableID byte, cuerpo []byte) []byte {
	largo := len(cuerpo) + 4
	//nolint:gosec
	s := []byte{tableID, 0xb0 | byte(largo>>8), byte(largo)}
	s = append(s, cuerpo...)
	return append(s, 0, 0, 0, 0)
}

func segmentoTS(tipos ...byte) []byte {
	pat := seccionPSI(0x00, []byte{0x00, 0x01, 0xc1, 0x00, 0x00, 0x00, 0x01, 0xf0, 0x00}) // PMT en PID 0x1000
	cuerpo := []byte{0x00, 0x01, 0xc1, 0x00, 0x00, 0xe1, 0x00, 0xf0, 0x00}
	for i, tipo := range tipos {
		cuerpo = append(cuerpo, tipo, 0xe1, byte(i), 0xf0, 0x00)
	}
	out := append([]byte{}, paqueteTS(0, pat)...)
	return append(out, paqueteTS(0x1000, seccionPSI(0x02, cuerpo))...)
}

type origen struct {
	srv          *httptest.Server
	hits         map[string]*atomic.Int32
	cabeceras    map[string]http.Header
	segmento     []byte
	ignoraRange  bool
	cuerpoGrande bool
	conMap       bool
}

func nuevoOrigen(t *testing.T) *origen {
	t.Helper()
	o := &origen{hits: map[string]*atomic.Int32{}, cabeceras: map[string]http.Header{}, segmento: segmentoTS(0x02, 0x03)}
	for _, ruta := range []string{"/master.m3u8", "/media.m3u8", "/seg.ts"} {
		o.hits[ruta] = &atomic.Int32{}
	}
	o.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h, ok := o.hits[r.URL.Path]; ok {
			h.Add(1)
		}
		o.cabeceras[r.URL.Path] = r.Header.Clone()
		switch r.URL.Path {
		case "/master.m3u8":
			w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
			_, _ = w.Write([]byte("#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=5000000,RESOLUTION=1920x1080\nmedia.m3u8\n"))
		case "/media.m3u8":
			w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
			cuerpo := "#EXTM3U\n#EXT-X-TARGETDURATION:4\n#EXT-X-MEDIA-SEQUENCE:1\n"
			if o.conMap {
				cuerpo += "#EXT-X-MAP:URI=\"init.mp4\"\n"
			}
			cuerpo += "#EXTINF:4.0,\nseg.ts\n#EXTINF:4.0,\nseg2.ts\n"
			_, _ = w.Write([]byte(cuerpo))
		case "/seg.ts":
			w.Header().Set("Content-Type", "video/MP2T")
			if o.cuerpoGrande {
				// 4 MB de relleno TS válido tras la PMT: el cliente debe cortar a 16 KB.
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(o.segmento)
				relleno := make([]byte, 188)
				relleno[0] = 0x47
				relleno[1] = 0x01
				relleno[2] = 0x00
				relleno[3] = 0x10
				for i := 0; i < 4<<20/188; i++ {
					if _, err := w.Write(relleno); err != nil {
						return
					}
				}
				return
			}
			if rango := r.Header.Get("Range"); rango != "" && !o.ignoraRange {
				w.Header().Set("Content-Range", "bytes 0-375/4623108")
				w.WriteHeader(http.StatusPartialContent)
			}
			_, _ = w.Write(o.segmento)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(o.srv.Close)
	return o
}

func checkerDeTest() *validator.Checker {
	return validator.NewCheckerConSonda(nil, proxy.NuevoClienteGuardado(true), 3*time.Second)
}

func TestSondaMasterMediaSegmentoClasificaMPEG2(t *testing.T) {
	o := nuevoOrigen(t)
	res := checkerDeTest().CheckTarea(context.Background(), validator.TareaCheck{URL: o.srv.URL + "/master.m3u8", CodecCaducado: true})
	if !res.IsAlive {
		t.Fatalf("vivo esperado: %v", res.Error)
	}
	if !res.CodecSondeado || res.Codec != domain.CodecNo || res.Codecs != "mpeg2video,mp2" {
		t.Errorf("sondeado=%v codec=%v codecs=%q", res.CodecSondeado, res.Codec, res.Codecs)
	}
	if o.hits["/media.m3u8"].Load() != 1 || o.hits["/seg.ts"].Load() != 1 {
		t.Errorf("hits media=%d seg=%d, quiero 1 y 1", o.hits["/media.m3u8"].Load(), o.hits["/seg.ts"].Load())
	}
	if rango := o.cabeceras["/seg.ts"].Get("Range"); rango != "bytes=0-16383" {
		t.Errorf("Range = %q", rango)
	}
}

func TestSondaMediaPlaylistDirectaH264(t *testing.T) {
	o := nuevoOrigen(t)
	o.segmento = segmentoTS(0x1b, 0x0f)
	res := checkerDeTest().CheckTarea(context.Background(), validator.TareaCheck{URL: o.srv.URL + "/media.m3u8", CodecCaducado: true})
	if res.Codec != domain.CodecOK || res.Codecs != "h264,aac" {
		t.Errorf("codec=%v codecs=%q", res.Codec, res.Codecs)
	}
	if o.hits["/master.m3u8"].Load() != 0 {
		t.Error("una media playlist no debe provocar ningún salto a un master")
	}
}

func TestSondaOrigenQueIgnoraRangeSeCortaA16KB(t *testing.T) {
	o := nuevoOrigen(t)
	o.cuerpoGrande = true
	inicio := time.Now()
	res := checkerDeTest().CheckTarea(context.Background(), validator.TareaCheck{URL: o.srv.URL + "/media.m3u8", CodecCaducado: true})
	if res.Codec != domain.CodecNo {
		t.Errorf("codec=%v; con la PMT en los primeros bytes debe clasificar aunque el origen ignore el Range", res.Codec)
	}
	if time.Since(inicio) > 2*time.Second {
		t.Errorf("tardó %v: no puede estar leyendo los 4 MB", time.Since(inicio))
	}
}

func TestSondaFMP4QuedaDesconocidoPeroSondeado(t *testing.T) {
	o := nuevoOrigen(t)
	o.conMap = true
	res := checkerDeTest().CheckTarea(context.Background(), validator.TareaCheck{URL: o.srv.URL + "/media.m3u8", CodecCaducado: true})
	if !res.CodecSondeado || res.Codec != domain.CodecUnknown {
		t.Errorf("sondeado=%v codec=%v", res.CodecSondeado, res.Codec)
	}
	if o.hits["/seg.ts"].Load() != 0 {
		t.Error("con EXT-X-MAP no se pide ningún segmento")
	}
}

// El segmento lo dicta el manifiesto: con la guardia activa (producción),
// un segmento en loopback no se descarga y el veredicto es "no se sabe".
func TestSondaSegmentoEnDestinoPrivadoQuedaDesconocido(t *testing.T) {
	o := nuevoOrigen(t)
	c := validator.NewCheckerConSonda(nil, proxy.NuevoClienteGuardado(false), 3*time.Second)
	res := c.CheckTarea(context.Background(), validator.TareaCheck{URL: o.srv.URL + "/media.m3u8", CodecCaducado: true})
	if !res.IsAlive {
		t.Fatal("el manifiesto va por el cliente de siempre y debe dar vivo")
	}
	if res.Codec != domain.CodecUnknown || !res.CodecSondeado {
		t.Errorf("codec=%v sondeado=%v", res.Codec, res.CodecSondeado)
	}
	if o.hits["/seg.ts"].Load() != 0 {
		t.Error("la guardia debe cortar ANTES de conectar")
	}
}

func TestSondaMandaCabecerasDelStreamEnLosDosSaltos(t *testing.T) {
	o := nuevoOrigen(t)
	checkerDeTest().CheckTarea(context.Background(), validator.TareaCheck{
		URL: o.srv.URL + "/master.m3u8", Referrer: "https://ref.example/", UserAgent: "UA-Propio/1.0", CodecCaducado: true,
	})
	for _, ruta := range []string{"/media.m3u8", "/seg.ts"} {
		h := o.cabeceras[ruta]
		if h.Get("Referer") != "https://ref.example/" || h.Get("User-Agent") != "UA-Propio/1.0" {
			t.Errorf("%s: Referer=%q UA=%q", ruta, h.Get("Referer"), h.Get("User-Agent"))
		}
	}
}

func TestSondaNoCorreSiElVeredictoNoEstaCaducado(t *testing.T) {
	o := nuevoOrigen(t)
	res := checkerDeTest().CheckTarea(context.Background(), validator.TareaCheck{URL: o.srv.URL + "/master.m3u8", CodecCaducado: false})
	if !res.IsAlive || res.CodecSondeado {
		t.Errorf("vivo=%v sondeado=%v", res.IsAlive, res.CodecSondeado)
	}
	if o.hits["/media.m3u8"].Load() != 0 || o.hits["/seg.ts"].Load() != 0 {
		t.Error("sin caducidad no hay peticiones extra")
	}
}

func TestCheckConCabecerasNuncaSondea(t *testing.T) {
	o := nuevoOrigen(t)
	res := checkerDeTest().Check(context.Background(), o.srv.URL+"/master.m3u8")
	if res.CodecSondeado || o.hits["/seg.ts"].Load() != 0 {
		t.Error("Check/CheckConCabeceras conservan el comportamiento de siempre")
	}
}
