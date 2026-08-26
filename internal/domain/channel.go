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

// Channel es además el contrato de wire con la app Flutter: los tags JSON
// reproducen los nombres de campo de Go que la app ya lee (json['ID'],
// json['LogoURL']…). Son explícitos a propósito — sin ellos, renombrar un
// campo compila limpio y rompe la app en silencio.
// Ver internal/domain/channel_test.go.
type Channel struct {
	ID ChannelID `json:"ID"`
	// TvgID es el tvg-id del M3U. Nació para unir el canal con su EPG, y esa
	// guía se retiró — pero NO se puede borrar: opensource.countryFromTvgID lo
	// usa para derivar CountryCode (formato "Nombre.cc@Feed"), y 10 835 de
	// 12 639 canales obtienen su país por esa vía. Sin él, el filtro de países
	// se queda casi vacío.
	TvgID        string `json:"TvgID"`
	Name         string `json:"Name"`
	LogoURL      string `json:"LogoURL"`
	CategoryID   string `json:"CategoryID"`
	LanguageCode string `json:"LanguageCode"` // ISO 639-1
	CountryCode  string `json:"CountryCode"`  // ISO 3166-1 alpha-2
	ProviderID   string `json:"ProviderID"`
	// Salud agregada de los streams del canal, rellenada por FindFiltered
	// para que la lista pinte el indicador sin N+1 a /channels/{id}/health.
	// Alive nil = ningún stream chequeado aún.
	Alive     *bool `json:"Alive"`
	LatencyMs int64 `json:"LatencyMs"` // mejor latencia entre streams vivos; 0 si no aplica
	// WebOK dice si el canal se reproduce DIRECTAMENTE en un navegador
	// (veredicto estricto: HTTPS + CORS + códecs de navegador). nil = ningún
	// stream comprobado aún. Campo ADITIVO: la app Flutter lo ignora, porque
	// Channel.fromJson lee claves por nombre.
	WebOK        *bool        `json:"WebOK"`
	ProviderType ProviderType `json:"ProviderType"`
	IsAdult      bool         `json:"IsAdult"`
	CreatedAt    time.Time    `json:"CreatedAt"`
	UpdatedAt    time.Time    `json:"UpdatedAt"`
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
