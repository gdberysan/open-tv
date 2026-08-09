package domain_test

import (
	"testing"

	"github.com/gdberysan/open-tv/gateway/internal/domain"
)

const masterSoportado = `#EXTM3U
#EXT-X-STREAM-INF:BANDWIDTH=2000000,CODECS="avc1.4d4028,mp4a.40.2"
720p.m3u8
`

const masterNoSoportado = `#EXTM3U
#EXT-X-STREAM-INF:BANDWIDTH=800000,CODECS="mp4v.20.9,mp4a.40.2"
low.m3u8
`

const masterMixto = `#EXTM3U
#EXT-X-STREAM-INF:BANDWIDTH=800000,CODECS="mp4v.20.9,mp4a.40.2"
low.m3u8
#EXT-X-STREAM-INF:BANDWIDTH=2000000,CODECS="avc1.64001f,mp4a.40.2"
high.m3u8
`

const masterSinCodecs = `#EXTM3U
#EXT-X-STREAM-INF:BANDWIDTH=2000000
720p.m3u8
`

// Una variante declarada y otra sin declarar: no se puede afirmar que el canal
// sea incompatible, porque la no declarada podría reproducirse.
const masterParcial = `#EXTM3U
#EXT-X-STREAM-INF:BANDWIDTH=800000,CODECS="mp4v.20.9"
low.m3u8
#EXT-X-STREAM-INF:BANDWIDTH=2000000
high.m3u8
`

const playlistDeMedios = `#EXTM3U
#EXT-X-TARGETDURATION:6
#EXTINF:6.000,
seg1.ts
`

const widevine = `#EXTM3U
#EXT-X-KEY:METHOD=SAMPLE-AES,KEYFORMAT="urn:uuid:edef8ba9-79d6-4ace-a3c8-27dcd51d21ed",URI="skd://x"
#EXT-X-STREAM-INF:BANDWIDTH=2000000,CODECS="avc1.4d4028,mp4a.40.2"
720p.m3u8
`

const fairplay = `#EXTM3U
#EXT-X-KEY:METHOD=SAMPLE-AES,KEYFORMAT="com.apple.streamingkeydelivery",URI="skd://x"
#EXT-X-STREAM-INF:BANDWIDTH=2000000,CODECS="avc1.4d4028,mp4a.40.2"
720p.m3u8
`

func TestClassifyManifest(t *testing.T) {
	casos := []struct {
		nombre string
		url    string
		body   string
		quiero domain.AirplaySupport
	}{
		{"h264+aac es reproducible", "http://x/a.m3u8", masterSoportado, domain.AirplayOK},
		{"mpeg-4 visual no lo es", "http://x/a.m3u8", masterNoSoportado, domain.AirplayNo},
		{"basta una variante buena", "http://x/a.m3u8", masterMixto, domain.AirplayOK},
		{"master sin CODECS no se puede juzgar", "http://x/a.m3u8", masterSinCodecs, domain.AirplayUnknown},
		{"una variante sin declarar impide el veredicto negativo", "http://x/a.m3u8", masterParcial, domain.AirplayUnknown},
		{"playlist de medios no declara nada", "http://x/a.m3u8", playlistDeMedios, domain.AirplayUnknown},
		{"widevine no viaja a AirPlay", "http://x/a.m3u8", widevine, domain.AirplayNo},
		{"fairplay sí es de Apple", "http://x/a.m3u8", fairplay, domain.AirplayOK},
		{"dash queda descartado por la url", "http://x/a.mpd", masterSoportado, domain.AirplayNo},
		{"rtmp queda descartado por la url", "rtmp://x/live", "", domain.AirplayNo},
		{"mmsh queda descartado por la url", "mmsh://x/live", "", domain.AirplayNo},
		{"cuerpo vacío no dice nada", "http://x/a.m3u8", "", domain.AirplayUnknown},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if got := domain.ClassifyManifest(c.url, c.body); got != c.quiero {
				t.Errorf("ClassifyManifest = %v, quiero %v", got, c.quiero)
			}
		})
	}
}
