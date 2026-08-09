package ports_test

import (
	"encoding/json"
	"testing"

	"github.com/gdberysan/open-tv/gateway/internal/ports"
)

// Los selectores de país y categoría de la app leen estas claves literalmente
// (mobile/lib/data/repositories/facet_repository.dart). Como Faceta no lleva
// etiquetas json, las claves son los nombres de campo de Go: renombrar Valor
// vaciaría los dos selectores en silencio. Este test es ese contrato, igual que
// el de domain.Channel.
func TestFacetaJSONCongelaElContratoConLaApp(t *testing.T) {
	raw, err := json.Marshal(ports.Faceta{Valor: "MX", Count: 120})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if got["Valor"] != "MX" {
		t.Errorf("Valor = %v, quiero \"MX\"; la app lo lee con esa clave exacta", got["Valor"])
	}
	if got["Count"] != float64(120) {
		t.Errorf("Count = %v, quiero 120", got["Count"])
	}
	if len(got) != 2 {
		t.Errorf("el JSON tiene %d claves, quiero 2", len(got))
	}
}
