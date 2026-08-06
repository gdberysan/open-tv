package domain

import (
	"fmt"
	"time"
)

type ChannelID string
type ProviderType string

const (
	ProviderOpenSource ProviderType = "opensource"
)

type Channel struct {
	ID ChannelID
	// TvgID es el identificador XMLTV (tvg-id del M3U); une el canal con su EPG.
	TvgID        string
	Name         string
	LogoURL      string
	CategoryID   string
	LanguageCode string // ISO 639-1
	CountryCode  string // ISO 3166-1 alpha-2
	ProviderID   string
	// Salud agregada de los streams del canal, rellenada por FindFiltered
	// para que la lista pinte el indicador sin N+1 a /channels/{id}/health.
	// Alive nil = ningún stream chequeado aún.
	Alive        *bool
	LatencyMs    int64 // mejor latencia entre streams vivos; 0 si no aplica
	ProviderType ProviderType
	IsAdult      bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (c Channel) Validate() error {
	if c.ID == "" {
		return fmt.Errorf("channel.Validate: ID requerido")
	}
	if c.Name == "" {
		return fmt.Errorf("channel.Validate: Name requerido")
	}
	if c.ProviderType == "" {
		return fmt.Errorf("channel.Validate: ProviderType requerido")
	}
	return nil
}
