package domain

import "time"

type EPGEntry struct {
	ChannelID   ChannelID
	Title       string
	Description string
	StartAt     time.Time
	EndAt       time.Time
}
