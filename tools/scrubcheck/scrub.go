// Package main implementa scrubcheck: el gate de release que garantiza que
// no viaja PII del autor al árbol público de open-tv.
//
// Este fichero contiene la lógica pura (sin git ni FS) para que sea testeable
// sin depender del entorno: el matcher de la denylist privada (findDenied) y
// las reglas duras que se aplican siempre, tenga o no denylist (reglasDuras).
package main

import (
	"path/filepath"
	"strings"
)

// Kind identifica el origen de un Hit, para que quien imprime sepa si puede
// nombrar el detalle (regla dura) o debe ocultarlo (denylist == PII).
const (
	KindDenylist  = "denylist"
	KindReglaDura = "regla-dura"
)

// Fichero es un fichero versionado con su ruta relativa y su contenido.
type Fichero struct {
	Path    string
	Content string
}

// Hit es un hallazgo del escaneo. Detail lleva el término que matcheó
// (KindDenylist) o una descripción de la regla (KindReglaDura). Quien
// imprime NUNCA debe mostrar Detail para KindDenylist: es PII.
type Hit struct {
	Path   string
	Kind   string
	Detail string
}

// findDenied replica el matcher de CVForge (lib/release/scrub.ts): término
// >=3 chars tras trim+lower, substring case-insensitive sobre el contenido.
// Términos vacíos o <3 chars se ignoran (evita falsos positivos masivos por
// abreviaturas de 1-2 letras).
func findDenied(files []Fichero, terms []string) []Hit {
	cleaned := make([]string, 0, len(terms))
	for _, t := range terms {
		t = strings.ToLower(strings.TrimSpace(t))
		if len(t) < 3 {
			continue
		}
		cleaned = append(cleaned, t)
	}

	var hits []Hit
	for _, f := range files {
		haystack := strings.ToLower(f.Content)
		for _, term := range cleaned {
			if strings.Contains(haystack, term) {
				hits = append(hits, Hit{Path: f.Path, Kind: KindDenylist, Detail: term})
			}
		}
	}
	return hits
}

// reglasDuras aplica las reglas de release independientes de la denylist:
// nunca debe viajar al árbol público un .DS_Store, una base de datos (*.db),
// nada bajo docs/prompts/, ni .claude/settings.local.json. Se evalúan sobre
// las rutas tal como las devuelve `git ls-files` (separador '/').
func reglasDuras(paths []string) []Hit {
	var hits []Hit
	for _, p := range paths {
		norm := filepath.ToSlash(p)

		switch {
		case filepath.Base(norm) == ".DS_Store":
			hits = append(hits, Hit{Path: p, Kind: KindReglaDura, Detail: ".DS_Store versionado"})
		case strings.HasSuffix(strings.ToLower(norm), ".db"):
			hits = append(hits, Hit{Path: p, Kind: KindReglaDura, Detail: "base de datos (*.db) versionada"})
		case strings.HasPrefix(norm, "docs/prompts/"):
			hits = append(hits, Hit{Path: p, Kind: KindReglaDura, Detail: "ruta bajo docs/prompts/ versionada"})
		case norm == ".claude/settings.local.json":
			hits = append(hits, Hit{Path: p, Kind: KindReglaDura, Detail: ".claude/settings.local.json versionado"})
		}
	}
	return hits
}
