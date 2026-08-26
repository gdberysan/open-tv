package main

import "testing"

func TestFindDenied_HitEnContenido(t *testing.T) {
	files := []Fichero{
		{Path: "README.md", Content: "contacto: correo-privado@example.com"},
		{Path: "notas.txt", Content: "nada relevante aquí"},
	}
	hits := findDenied(files, []string{"correo-privado@example.com"})

	if len(hits) != 1 {
		t.Fatalf("se esperaba 1 hallazgo, hubo %d: %+v", len(hits), hits)
	}
	if hits[0].Path != "README.md" {
		t.Errorf("path del hallazgo = %q, se esperaba README.md", hits[0].Path)
	}
	if hits[0].Kind != KindDenylist {
		t.Errorf("kind del hallazgo = %q, se esperaba %q", hits[0].Kind, KindDenylist)
	}
}

func TestFindDenied_SinFalsoPositivo(t *testing.T) {
	files := []Fichero{
		{Path: "README.md", Content: "documentación pública sin nada sensible"},
	}
	hits := findDenied(files, []string{"correo-privado@example.com", "usuario"})

	if len(hits) != 0 {
		t.Fatalf("no se esperaban hallazgos, hubo %d: %+v", len(hits), hits)
	}
}

func TestFindDenied_CaseInsensitive(t *testing.T) {
	files := []Fichero{
		{Path: "a.go", Content: "// autor: usuario"},
	}
	hits := findDenied(files, []string{"usuario"})

	if len(hits) != 1 {
		t.Fatalf("se esperaba 1 hallazgo (case-insensitive), hubo %d", len(hits))
	}
}

func TestFindDenied_SubstringCualquierParte(t *testing.T) {
	files := []Fichero{
		{Path: "b.go", Content: "usuario=usuario-mac"},
	}
	hits := findDenied(files, []string{"usuario"})

	if len(hits) != 1 {
		t.Fatalf("se esperaba 1 hallazgo (substring), hubo %d", len(hits))
	}
}

func TestFindDenied_IgnoraTerminosCortosOVacios(t *testing.T) {
	files := []Fichero{
		{Path: "c.go", Content: "gb es una abreviatura frecuente, igual que ''"},
	}
	hits := findDenied(files, []string{"gb", "", "  ", "yz"})

	if len(hits) != 0 {
		t.Fatalf("términos <3 chars o vacíos no deben producir hallazgos, hubo %d: %+v", len(hits), hits)
	}
}

func TestFindDenied_NormalizaEspaciosYMayusculas(t *testing.T) {
	files := []Fichero{
		{Path: "d.go", Content: "contiene GDBERYSAN en el texto"},
	}
	hits := findDenied(files, []string{"  GdBerysan  "})

	if len(hits) != 1 {
		t.Fatalf("se esperaba 1 hallazgo tras normalizar el término, hubo %d", len(hits))
	}
}

func TestReglasDuras_DetectaDSStore(t *testing.T) {
	hits := reglasDuras([]string{"foo/.DS_Store", ".DS_Store", "internal/ok.go"})

	dsHits := contarPorPath(hits, "foo/.DS_Store")
	if dsHits != 1 {
		t.Errorf("se esperaba 1 hallazgo para foo/.DS_Store, hubo %d", dsHits)
	}
	dsHits = contarPorPath(hits, ".DS_Store")
	if dsHits != 1 {
		t.Errorf("se esperaba 1 hallazgo para .DS_Store, hubo %d", dsHits)
	}
	if contarPorPath(hits, "internal/ok.go") != 0 {
		t.Errorf("internal/ok.go no debería marcarse")
	}
}

func TestReglasDuras_DetectaBasesDeDatos(t *testing.T) {
	hits := reglasDuras([]string{"x.db", "data/canales.db", "internal/ok.go"})

	if contarPorPath(hits, "x.db") != 1 {
		t.Errorf("x.db debería marcarse como regla dura")
	}
	if contarPorPath(hits, "data/canales.db") != 1 {
		t.Errorf("data/canales.db debería marcarse como regla dura")
	}
	if contarPorPath(hits, "internal/ok.go") != 0 {
		t.Errorf("internal/ok.go no debería marcarse")
	}
}

func TestReglasDuras_DetectaDocsPrompts(t *testing.T) {
	hits := reglasDuras([]string{"docs/prompts/algo.md", "docs/prompts/sub/otro.txt", "docs/otros/ok.md"})

	if contarPorPath(hits, "docs/prompts/algo.md") != 1 {
		t.Errorf("docs/prompts/algo.md debería marcarse")
	}
	if contarPorPath(hits, "docs/prompts/sub/otro.txt") != 1 {
		t.Errorf("cualquier ruta bajo docs/prompts/ debería marcarse")
	}
	if contarPorPath(hits, "docs/otros/ok.md") != 0 {
		t.Errorf("docs/otros/ok.md no debería marcarse")
	}
}

func TestReglasDuras_DetectaSettingsLocal(t *testing.T) {
	hits := reglasDuras([]string{".claude/settings.local.json", ".claude/settings.json"})

	if contarPorPath(hits, ".claude/settings.local.json") != 1 {
		t.Errorf(".claude/settings.local.json debería marcarse")
	}
	if contarPorPath(hits, ".claude/settings.json") != 0 {
		t.Errorf(".claude/settings.json NO debería marcarse")
	}
}

func TestReglasDuras_NoMarcaFicherosNormales(t *testing.T) {
	hits := reglasDuras([]string{
		"main.go",
		"internal/ui/dist/index.html",
		"web/package.json",
		"README.md",
	})

	if len(hits) != 0 {
		t.Fatalf("no se esperaban hallazgos en ficheros normales, hubo %d: %+v", len(hits), hits)
	}
}

func TestReglasDuras_MarcanKindReglaDura(t *testing.T) {
	hits := reglasDuras([]string{".DS_Store"})

	if len(hits) != 1 {
		t.Fatalf("se esperaba 1 hallazgo")
	}
	if hits[0].Kind != KindReglaDura {
		t.Errorf("kind = %q, se esperaba %q", hits[0].Kind, KindReglaDura)
	}
}

func contarPorPath(hits []Hit, path string) int {
	n := 0
	for _, h := range hits {
		if h.Path == path {
			n++
		}
	}
	return n
}
