package domain

import "time"

type Protocol string

const (
	ProtocolHLS  Protocol = "HLS"
	ProtocolDASH Protocol = "DASH"
	ProtocolRTMP Protocol = "RTMP"
)

// Tags JSON explícitos por el mismo motivo que en Channel: congelan los
// nombres que verían los clientes en vez de dejarlos atados al nombre del
// campo de Go.
type Stream struct {
	ID        string    `json:"ID"`
	ChannelID ChannelID `json:"ChannelID"`
	URL       string    `json:"URL"`
	Protocol  Protocol  `json:"Protocol"`
	// Referrer y UserAgent son las cabeceras que el ORIGEN exige para servir
	// este stream (las publica la API de iptv-org). Vacías = usar las de
	// siempre. El navegador no puede ponerlas —Referer y User-Agent son
	// cabeceras prohibidas para fetch/XHR—, así que solo las usan el proxy de
	// loopback y el health-checker.
	Referrer    string    `json:"Referrer"`
	UserAgent   string    `json:"UserAgent"`
	LatencyMs   int64     `json:"LatencyMs"`
	IsAlive     bool      `json:"IsAlive"`
	LastChecked time.Time `json:"LastChecked"`
}
