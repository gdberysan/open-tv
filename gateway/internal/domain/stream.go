package domain

import "time"

type Protocol string

const (
	ProtocolHLS  Protocol = "HLS"
	ProtocolDASH Protocol = "DASH"
	ProtocolRTMP Protocol = "RTMP"
)

type Stream struct {
	ID          string
	ChannelID   ChannelID
	URL         string
	Protocol    Protocol
	LatencyMs   int64
	IsAlive     bool
	LastChecked time.Time
}
