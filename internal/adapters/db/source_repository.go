package db

import (
	"context"
	"database/sql"
	"fmt"
	"hash/fnv"
	"strings"
	"time"

	"github.com/gdberysan/open-tv/internal/ports"
)

var _ ports.SourceRepository = (*SQLiteSourceRepository)(nil)

// SQLiteSourceRepository implementa ports.SourceRepository sobre la misma
// tabla providers que antes solo alojaba el proveedor IPTV-org fijo. type
// sigue constante a 'opensource': lo que distingue una fuente de otra es
// base_url (y, dentro de una URL, kind: remota vs fichero subido).
type SQLiteSourceRepository struct {
	db *sql.DB
}

func NewSourceRepository(db *sql.DB) *SQLiteSourceRepository {
	return &SQLiteSourceRepository{db: db}
}

// sourceID deriva un ID determinista de la URL: la misma URL añadida dos
// veces produce el mismo ID, lo que convierte el UNIQUE de providers.id en la
// guarda natural contra duplicados. Mismo esquema que streamID en el Syncer
// (fnv64a, "%016x"), con el prefijo "src-" en vez de "st-".
func sourceID(url string) string {
	h := fnv.New64a()
	// hash.Hash.Write nunca devuelve error (garantía de io.Writer para fnv):
	// se descarta explícitamente en vez de comprobarlo.
	_, _ = h.Write([]byte(url))
	return fmt.Sprintf("src-%016x", h.Sum64())
}

// List devuelve todas las fuentes dadas de alta, con Canales calculado en
// vivo (COUNT sobre channels) para que el número nunca descuadre del catálogo
// real. UltimoSync reutiliza providers.updated_at (0 = nunca sincronizada,
// ver Add): no hace falta una columna nueva solo para ese timestamp.
func (r *SQLiteSourceRepository) List(ctx context.Context) ([]ports.Source, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT p.id, p.label, p.base_url, p.kind, p.is_active, p.updated_at, p.tvg_url,
		       (SELECT COUNT(*) FROM channels c WHERE c.provider_id = p.id) AS canales
		FROM providers p
		ORDER BY p.created_at, p.id
	`)
	if err != nil {
		return nil, fmt.Errorf("db.SourceRepository.List: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []ports.Source
	for rows.Next() {
		var (
			s        ports.Source
			isActive int
		)
		if err := rows.Scan(&s.ID, &s.Label, &s.URL, &s.Kind, &isActive, &s.UltimoSync, &s.TvgURL, &s.Canales); err != nil {
			return nil, fmt.Errorf("db.SourceRepository.List (scan): %w", err)
		}
		s.IsActive = isActive == 1
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db.SourceRepository.List (rows.Err): %w", err)
	}
	return out, nil
}

// Add da de alta una fuente nueva. El ID lo deriva sourceID a partir de la
// URL, así que el ID que traiga s se ignora: es la propia URL la que decide
// si es una fuente ya conocida. updated_at arranca en 0 (nunca sincronizada)
// y NO en "now": si arrancara en la hora de alta, List reportaría un
// UltimoSync falso antes de que el Syncer la toque por primera vez.
func (r *SQLiteSourceRepository) Add(ctx context.Context, s ports.Source) error {
	id := sourceID(s.URL)
	now := time.Now().Unix()

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO providers (id, type, base_url, label, kind, priority, is_active, created_at, updated_at)
		VALUES (?, 'opensource', ?, ?, ?, 100, ?, ?, 0)
	`, id, s.URL, s.Label, s.Kind, boolToInt(s.IsActive), now)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return fmt.Errorf("db.SourceRepository.Add (url=%s): %w", s.URL, ports.ErrFuenteDuplicada)
		}
		return fmt.Errorf("db.SourceRepository.Add (url=%s): %w", s.URL, err)
	}
	return nil
}

// Remove borra la fuente. channels.provider_id y streams.channel_id están
// declarados con ON DELETE CASCADE (ver schema.sql), así que este único
// DELETE arrastra en cascada los canales de ESTE provider y, tras ellos, sus
// streams — sin tocar los de ningún otro provider, porque la cascada está
// scoped por la propia FK (provider_id = id borrado).
func (r *SQLiteSourceRepository) Remove(ctx context.Context, id string) error {
	if _, err := r.db.ExecContext(ctx, "DELETE FROM providers WHERE id = ?", id); err != nil {
		return fmt.Errorf("db.SourceRepository.Remove (id=%s): %w", id, err)
	}
	return nil
}

// TouchSync sella el momento del último sync exitoso reutilizando
// providers.updated_at.
func (r *SQLiteSourceRepository) TouchSync(ctx context.Context, id string, cuando int64) error {
	if _, err := r.db.ExecContext(ctx,
		"UPDATE providers SET updated_at = ? WHERE id = ?", cuando, id); err != nil {
		return fmt.Errorf("db.SourceRepository.TouchSync (id=%s): %w", id, err)
	}
	return nil
}

// SetTvgURL fija la url-tvg que la fuente declaró en su cabecera M3U (ver
// opensource.Provider.TvgURLs). List la refleja después en Source.TvgURL.
func (r *SQLiteSourceRepository) SetTvgURL(ctx context.Context, id string, tvgURL string) error {
	if _, err := r.db.ExecContext(ctx,
		"UPDATE providers SET tvg_url = ? WHERE id = ?", tvgURL, id); err != nil {
		return fmt.Errorf("db.SourceRepository.SetTvgURL (id=%s): %w", id, err)
	}
	return nil
}
