// Command scrubcheck es el gate de release de open-tv: garantiza que nada
// del árbol VERSIONADO (lo que un checkout público lee, vía `git ls-files`,
// no `git archive`) contiene PII del autor ni ficheros que nunca deberían
// haberse versionado.
//
// Contrato de salida (espeja lib/release/scrub.ts de CVForge):
//
//	0 = limpio
//	1 = hay hallazgos
//	2 = no hay denylist privada disponible (solo en modo con denylist)
//
// Dos modos:
//   - Por defecto: aplica reglas duras + la denylist privada
//     (~/.config/korven/open-tv-denylist.txt, o $KORVEN_DENYLIST). Gate LOCAL
//     del autor antes de un release; si la denylist no existe, exit 2.
//   - --solo-reglas-duras / -hard-only: solo las reglas duras, sin denylist.
//     Nunca sale 2. Es el modo que corre en CI, donde la denylist privada no
//     existe (y no debe existir).
//
// Privacidad al imprimir: un hallazgo de la denylist NUNCA imprime el
// término (es PII) — solo el fichero y "coincidencia con la lista privada".
// Los hallazgos de reglas duras sí nombran fichero y regla, porque no son
// secretos: son nombres de fichero prohibidos por política, no datos
// personales.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const denylistEnvVar = "KORVEN_DENYLIST"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr *os.File) int {
	hardOnly := false
	for _, a := range args {
		switch a {
		case "--solo-reglas-duras", "-hard-only", "--hard-only":
			hardOnly = true
		default:
			fmt.Fprintf(stderr, "scrubcheck: flag desconocido %q\n", a)
			return 1
		}
	}

	root, err := repoRoot()
	if err != nil {
		fmt.Fprintf(stderr, "scrubcheck: no se pudo localizar la raíz del repo: %v\n", err)
		return 1
	}

	paths, err := gitLsFiles(root)
	if err != nil {
		fmt.Fprintf(stderr, "scrubcheck: git ls-files falló: %v\n", err)
		return 1
	}

	var hits []Hit
	hits = append(hits, reglasDuras(paths)...)

	if !hardOnly {
		denylistPath := resolveDenylistPath()
		terms, err := loadDenylist(denylistPath)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Fprintf(stderr, "scrubcheck: denylist privada no encontrada en %s (define %s para otra ruta)\n", denylistPath, denylistEnvVar)
				return 2
			}
			fmt.Fprintf(stderr, "scrubcheck: no se pudo leer la denylist %s: %v\n", denylistPath, err)
			return 2
		}

		files, err := leerFicheros(root, paths)
		if err != nil {
			fmt.Fprintf(stderr, "scrubcheck: no se pudieron leer ficheros del árbol: %v\n", err)
			return 1
		}

		hits = append(hits, findDenied(files, terms)...)
	}

	if len(hits) == 0 {
		fmt.Fprintln(stdout, "scrubcheck: limpio — nada versionado dispara las reglas duras"+modoDenylistTexto(hardOnly))
		return 0
	}

	for _, h := range hits {
		imprimirHit(stdout, h)
	}
	fmt.Fprintf(stdout, "scrubcheck: %d hallazgo(s)\n", len(hits))
	return 1
}

func modoDenylistTexto(hardOnly bool) string {
	if hardOnly {
		return ""
	}
	return " ni la denylist privada"
}

func imprimirHit(w *os.File, h Hit) {
	switch h.Kind {
	case KindDenylist:
		// NUNCA se imprime h.Detail (el término) aquí: es PII.
		fmt.Fprintf(w, "%s: coincidencia con la lista privada\n", h.Path)
	default:
		fmt.Fprintf(w, "%s: regla dura (%s)\n", h.Path, h.Detail)
	}
}

// repoRoot localiza la raíz del repositorio git desde el directorio actual.
func repoRoot() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// gitLsFiles lista las rutas versionadas del árbol actual (el índice de git,
// que en un checkout limpio de HEAD es lo que un repo público expone) —
// deliberadamente NO usa `git archive`.
func gitLsFiles(root string) ([]string, error) {
	cmd := exec.Command("git", "ls-files")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	paths := make([]string, 0, len(lines))
	for _, l := range lines {
		if l == "" {
			continue
		}
		paths = append(paths, l)
	}
	return paths, nil
}

// leerFicheros lee del disco el contenido de cada ruta versionada, relativa
// a root. Un fichero binario o ilegible como texto se lee igualmente como
// bytes crudos (convertidos a string) — findDenied solo hace substring
// matching, no necesita que el contenido sea UTF-8 válido.
func leerFicheros(root string, paths []string) ([]Fichero, error) {
	files := make([]Fichero, 0, len(paths))
	for _, p := range paths {
		content, err := os.ReadFile(filepath.Join(root, p))
		if err != nil {
			// Un fichero listado por `git ls-files` pero ausente del disco
			// (p.ej. borrado sin `git rm`) no es un hallazgo de scrubcheck.
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("leyendo %s: %w", p, err)
		}
		files = append(files, Fichero{Path: p, Content: string(content)})
	}
	return files, nil
}

// resolveDenylistPath devuelve la ruta de la denylist privada: KORVEN_DENYLIST
// si está definida, si no ~/.config/korven/open-tv-denylist.txt.
func resolveDenylistPath() string {
	if p := os.Getenv(denylistEnvVar); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".config", "korven", "open-tv-denylist.txt")
	}
	return filepath.Join(home, ".config", "korven", "open-tv-denylist.txt")
}

// loadDenylist lee la denylist privada, una línea por término. Líneas vacías
// se ignoran aquí; el filtrado por longitud (<3 chars) lo hace findDenied.
func loadDenylist(path string) ([]string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var terms []string
	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		terms = append(terms, line)
	}
	return terms, nil
}
