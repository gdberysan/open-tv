package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/tu-org/iptv-ecosystem/gateway/internal/domain"
	"github.com/tu-org/iptv-ecosystem/gateway/internal/ports"
)

// SQLiteChannelRepository implementa ports.ChannelRepository sobre SQLite.
type SQLiteChannelRepository struct {
	db *sql.DB
}

func NewChannelRepository(db *sql.DB) *SQLiteChannelRepository {
	return &SQLiteChannelRepository{db: db}
}

const channelColumns = ` id, tvg_id, name, logo_url, category_id, language_code, country_code,
	provider_id, provider_type, is_adult, created_at, updated_at `

const upsertSQL = `
	INSERT INTO channels (id, tvg_id, name, logo_url, category_id, language_code, country_code,
	                      provider_id, provider_type, is_adult, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		tvg_id        = excluded.tvg_id,
		name          = excluded.name,
		logo_url      = excluded.logo_url,
		category_id   = excluded.category_id,
		language_code = excluded.language_code,
		country_code  = excluded.country_code,
		updated_at    = excluded.updated_at`

// nullStr convierte un string vacío a NULL de SQL. Crítico para FKs opcionales
// como category_id: SQLite permite NULL en una FK nullable, pero no string vacío.
func nullStr(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

func (r *SQLiteChannelRepository) Save(ctx context.Context, ch domain.Channel) error {
	now := time.Now().Unix()

	// Pre-crear categoría si está definida (FK categories(id) lo requiere)
	if ch.CategoryID != "" {
		if _, err := r.db.ExecContext(ctx, `
			INSERT INTO categories (id, name, provider_type, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?)
			ON CONFLICT(id) DO NOTHING
		`, ch.CategoryID, ch.CategoryID, string(ch.ProviderType), now, now); err != nil {
			return fmt.Errorf("db.Save (category %s): %w", ch.CategoryID, err)
		}
	}

	if _, err := r.db.ExecContext(ctx, upsertSQL,
		string(ch.ID), nullStr(ch.TvgID), ch.Name, nullStr(ch.LogoURL), nullStr(ch.CategoryID),
		nullStr(ch.LanguageCode), nullStr(ch.CountryCode),
		ch.ProviderID, string(ch.ProviderType),
		boolToInt(ch.IsAdult), now, now,
	); err != nil {
		return fmt.Errorf("db.Save (id=%s): %w", ch.ID, err)
	}
	return nil
}

// SaveBatch hace upsert de canales en una sola transacción con statement preparado.
// Diseñado para cargas de IPTV-org (9000+ canales) sin saturar SQLite.
// Pre-crea las categorías referenciadas para satisfacer la FK categories(id).
func (r *SQLiteChannelRepository) SaveBatch(ctx context.Context, channels []domain.Channel) error {
	if len(channels) == 0 {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("db.SaveBatch (BeginTx): %w", err)
	}
	defer tx.Rollback() //nolint:errcheck — Rollback es no-op si Commit tuvo éxito

	// Paso 1: upsert de categorías únicas referenciadas por estos canales.
	// Necesario porque channels.category_id tiene FK hacia categories.id.
	if err := upsertCategories(ctx, tx, channels); err != nil {
		return fmt.Errorf("db.SaveBatch (upsertCategories): %w", err)
	}

	// Paso 2: upsert de canales
	stmt, err := tx.PrepareContext(ctx, upsertSQL)
	if err != nil {
		return fmt.Errorf("db.SaveBatch (Prepare): %w", err)
	}
	defer stmt.Close()

	now := time.Now().Unix()
	for _, ch := range channels {
		if _, err := stmt.ExecContext(ctx,
			string(ch.ID), nullStr(ch.TvgID), ch.Name, nullStr(ch.LogoURL), nullStr(ch.CategoryID),
			nullStr(ch.LanguageCode), nullStr(ch.CountryCode),
			ch.ProviderID, string(ch.ProviderType),
			boolToInt(ch.IsAdult), now, now,
		); err != nil {
			return fmt.Errorf("db.SaveBatch (Exec id=%s): %w", ch.ID, err)
		}
	}

	return tx.Commit()
}

// upsertCategories crea las categorías únicas del lote si no existen.
// Evita FK violations en channels.category_id sin requerir pre-carga manual.
func upsertCategories(ctx context.Context, tx *sql.Tx, channels []domain.Channel) error {
	seen := make(map[string]domain.ProviderType, 32)
	for _, ch := range channels {
		if ch.CategoryID != "" {
			seen[ch.CategoryID] = ch.ProviderType
		}
	}
	if len(seen) == 0 {
		return nil
	}

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO categories (id, name, provider_type, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(id) DO NOTHING
	`)
	if err != nil {
		return fmt.Errorf("upsertCategories (Prepare): %w", err)
	}
	defer stmt.Close()

	now := time.Now().Unix()
	for catID, provType := range seen {
		if _, err := stmt.ExecContext(ctx, catID, catID, string(provType), now, now); err != nil {
			return fmt.Errorf("upsertCategories (Exec %s): %w", catID, err)
		}
	}
	return nil
}

func (r *SQLiteChannelRepository) FindByID(ctx context.Context, id domain.ChannelID) (domain.Channel, error) {
	row := r.db.QueryRowContext(ctx,
		"SELECT"+channelColumns+"FROM channels WHERE id = ?", string(id))
	ch, err := scanChannel(row)
	if err != nil {
		return domain.Channel{}, fmt.Errorf("db.FindByID (id=%s): %w", id, err)
	}
	return ch, nil
}

// FindFiltered aplica filtros combinados (país, categoría, calidad, texto) en una
// sola query dinámica. Es el método principal que usa el handler /channels.
func (r *SQLiteChannelRepository) FindFiltered(ctx context.Context, f ports.ChannelFilter) ([]domain.Channel, error) {
	f = f.Normalize()

	var where []string
	var args []any

	if f.Query != "" {
		where = append(where, "name LIKE ? COLLATE NOCASE")
		args = append(args, "%"+f.Query+"%")
	}
	if f.Country != "" {
		where = append(where, "country_code = ?")
		args = append(args, f.Country)
	}
	if f.Category != "" {
		where = append(where, "category_id = ?")
		args = append(args, f.Category)
	}
	if clause, qargs := qualityWhereClause(f.MinQuality); clause != "" {
		where = append(where, clause)
		args = append(args, qargs...)
	}

	whereSQL := "1=1"
	if len(where) > 0 {
		whereSQL = strings.Join(where, " AND ")
	}

	q := "SELECT" + channelColumns + "FROM channels WHERE " + whereSQL +
		" ORDER BY name COLLATE NOCASE LIMIT ? OFFSET ?"
	args = append(args, f.Limit, f.Offset)

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("db.FindFiltered: %w", err)
	}
	defer rows.Close()
	return scanChannels(rows)
}

// qualityPatterns mapea nivel de calidad mínimo a los tokens que aparecen en
// los nombres de canales de IPTV-org siempre entre paréntesis,
// p.ej. "BBC One (1080p) [Geo-blocked]" o "DW (4K)".
// Usar paréntesis evita falsos positivos como "24Kitchen" matcheando "4K".
var qualityPatterns = map[string][]string{
	"4k":  {"(4K)", "(2160p)", "(UHD)"},
	"fhd": {"(1080p)", "(FHD)", "(4K)", "(2160p)", "(UHD)"},
	"hd":  {"(720p)", "(1080p)", "(FHD)", "(4K)", "(2160p)", "(UHD)"},
}

func qualityWhereClause(minQuality string) (string, []any) {
	patterns, ok := qualityPatterns[minQuality]
	if !ok || len(patterns) == 0 {
		return "", nil
	}
	parts := make([]string, len(patterns))
	args := make([]any, len(patterns))
	for i, p := range patterns {
		parts[i] = "name LIKE ?"
		args[i] = "%" + p + "%"
	}
	return "(" + strings.Join(parts, " OR ") + ")", args
}

// Search devuelve canales cuyo nombre contiene query (case-insensitive).
// Con query vacío devuelve los primeros `limit` canales.
func (r *SQLiteChannelRepository) Search(ctx context.Context, query string, limit int) ([]domain.Channel, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if query == "" {
		rows, err = r.db.QueryContext(ctx,
			"SELECT"+channelColumns+"FROM channels LIMIT ?", limit)
	} else {
		rows, err = r.db.QueryContext(ctx,
			"SELECT"+channelColumns+"FROM channels WHERE name LIKE ? COLLATE NOCASE LIMIT ?",
			"%"+query+"%", limit)
	}
	if err != nil {
		return nil, fmt.Errorf("db.Search: %w", err)
	}
	defer rows.Close()
	return scanChannels(rows)
}

func (r *SQLiteChannelRepository) Delete(ctx context.Context, id domain.ChannelID) error {
	if _, err := r.db.ExecContext(ctx, "DELETE FROM channels WHERE id = ?", string(id)); err != nil {
		return fmt.Errorf("db.Delete (id=%s): %w", id, err)
	}
	return nil
}

// ── helpers ──────────────────────────────────────────────────────────────────

func scanChannel(row *sql.Row) (domain.Channel, error) {
	var (
		ch                                                    domain.Channel
		tvgID, logoURL, categoryID, languageCode, countryCode sql.NullString
		providerID                                            string
		isAdult                                               int
		createdAt, updatedAt                                  int64
	)
	err := row.Scan(
		(*string)(&ch.ID), &tvgID, &ch.Name, &logoURL, &categoryID,
		&languageCode, &countryCode,
		&providerID, (*string)(&ch.ProviderType),
		&isAdult, &createdAt, &updatedAt,
	)
	if err != nil {
		return domain.Channel{}, err
	}
	ch.TvgID = tvgID.String
	ch.LogoURL = logoURL.String
	ch.CategoryID = categoryID.String
	ch.LanguageCode = languageCode.String
	ch.CountryCode = countryCode.String
	ch.ProviderID = providerID
	ch.IsAdult = isAdult == 1
	ch.CreatedAt = time.Unix(createdAt, 0)
	ch.UpdatedAt = time.Unix(updatedAt, 0)
	return ch, nil
}

func scanChannels(rows *sql.Rows) ([]domain.Channel, error) {
	var channels []domain.Channel
	for rows.Next() {
		var (
			ch                                                    domain.Channel
			tvgID, logoURL, categoryID, languageCode, countryCode sql.NullString
			providerID                                            string
			isAdult                                               int
			createdAt, updatedAt                                  int64
		)
		if err := rows.Scan(
			(*string)(&ch.ID), &tvgID, &ch.Name, &logoURL, &categoryID,
			&languageCode, &countryCode,
			&providerID, (*string)(&ch.ProviderType),
			&isAdult, &createdAt, &updatedAt,
		); err != nil {
			return nil, fmt.Errorf("db.scanChannels: %w", err)
		}
		ch.TvgID = tvgID.String
		ch.LogoURL = logoURL.String
		ch.CategoryID = categoryID.String
		ch.LanguageCode = languageCode.String
		ch.CountryCode = countryCode.String
		ch.ProviderID = providerID
		ch.IsAdult = isAdult == 1
		ch.CreatedAt = time.Unix(createdAt, 0)
		ch.UpdatedAt = time.Unix(updatedAt, 0)
		channels = append(channels, ch)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db.scanChannels (rows.Err): %w", err)
	}
	return channels, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
