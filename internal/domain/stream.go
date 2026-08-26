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
	ID          string    `json:"ID"`
	ChannelID   ChannelID `json:"ChannelID"`
	URL         string    `json:"URL"`
	Protocol    Protocol  `json:"Protocol"`
	LatencyMs   int64     `json:"LatencyMs"`
	IsAlive     bool      `json:"IsAlive"`
	LastChecked time.Time `json:"LastChecked"`
}
