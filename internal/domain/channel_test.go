package domain_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/gdberysan/open-tv/internal/domain"
)

// La app Flutter lee estas claves literalmente (mobile/lib/domain/models/
// channel.dart). Este test es el contrato: si alguien renombra un campo del
// dominio, falla aquí y no en producción con una lista vacía.
func TestChannelJSONCongelaElContratoConLaApp(t *testing.T) {
	alive := true
	ch := domain.Channel{
		ID:           "opensource-BBC One",
		TvgID:        "BBCOne.uk",
		Name:         "BBC One (1080p)",
		LogoURL:      "http://logo",
		CategoryID:   "General",
		LanguageCode: "en",
		CountryCode:  "GB",
		ProviderID:   "opensource",
		Alive:        &alive,
		LatencyMs:    120,
		ProviderType: domain.ProviderOpenSource,
		IsAdult:      false,
		CreatedAt:    time.Unix(0, 0),
		UpdatedAt:    time.Unix(0, 0),
	}

	raw, err := json.Marshal(ch)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	for _, k := range []string{
		"ID", "TvgID", "Name", "LogoURL", "CategoryID", "LanguageCode",
		"CountryCode", "ProviderID", "Alive", "LatencyMs", "WebOK", "ProviderType",
		"IsAdult", "CreatedAt", "UpdatedAt",
	} {
		if _, ok := got[k]; !ok {
			t.Errorf("falta la clave %q en el JSON; la app Flutter la lee", k)
		}
	}
	if len(got) != 15 {
		t.Errorf("el JSON tiene %d claves, quiero 15: añadir un campo al dominio lo filtra al cable", len(got))
	}
}

// El campo nuevo es ADITIVO. La app Flutter (mobile/lib/domain/models/
// channel.dart) construye Channel leyendo claves por nombre con
// `json['X'] as T?`, así que una clave de más la ignora. Lo que la rompería es
// quitar o renombrar una de las 14 originales — de eso se encarga la lista de
// arriba.
func TestWebOKEsNullCuandoNoSeHaComprobado(t *testing.T) {
	raw, err := json.Marshal(domain.Channel{ID: "x", Name: "X", ProviderType: domain.ProviderOpenSource})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if v, ok := got["WebOK"]; !ok || v != nil {
		t.Errorf("WebOK = %v (presente=%v), quiero null", v, ok)
	}
}

func TestChannelValidate(t *testing.T) {
	casos := []struct {
		nombre  string
		ch      domain.Channel
		quiereE bool
	}{
		{"completo", domain.Channel{ID: "a", Name: "A", ProviderType: domain.ProviderOpenSource}, false},
		{"sin ID", domain.Channel{Name: "A", ProviderType: domain.ProviderOpenSource}, true},
		{"sin Name", domain.Channel{ID: "a", ProviderType: domain.ProviderOpenSource}, true},
		{"sin ProviderType", domain.Channel{ID: "a", Name: "A"}, true},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			err := c.ch.Validate()
			if (err != nil) != c.quiereE {
				t.Errorf("Validate() = %v, quiere error = %v", err, c.quiereE)
			}
		})
	}
}
