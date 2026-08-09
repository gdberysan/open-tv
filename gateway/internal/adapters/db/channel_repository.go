package db

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gdberysan/open-tv/gateway/internal/domain"
	"github.com/gdberysan/open-tv/gateway/internal/ports"
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

// last_seen_at se sella con el mismo `now` que updated_at en cada upsert. Es lo
// que permite a DeleteStale distinguir lo que apareció en el último sync de lo
// que el proveedor dejó de listar.
const upsertSQL = `
	INSERT INTO channels (id, tvg_id, name, logo_url, category_id, language_code, country_code,
	                      provider_id, provider_type, is_adult, created_at, updated_at, last_seen_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		tvg_id        = excluded.tvg_id,
		name          = excluded.name,
		logo_url      = excluded.logo_url,
		category_id   = excluded.category_id,
		language_code = excluded.language_code,
		country_code  = excluded.country_code,
		updated_at    = excluded.updated_at,
		last_seen_at  = excluded.last_seen_at`

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
		boolToInt(ch.IsAdult), now, now, now,
	); err != nil {
		return fmt.Errorf("db.Save (id=%s): %w", ch.ID, err)
	}
	return nil
}

// DeleteStale borra los canales del proveedor que no aparecieron en el último
// sync exitoso. El ID se deriva del nombre, así que un renombrado aguas arriba
// deja una fila huérfana que nunca se podría reproducir: el proveedor ya no la
// conoce y /channels/stream devuelve 404. Los streams caen por ON DELETE
// CASCADE.
func (r *SQLiteChannelRepository) DeleteStale(ctx context.Context, providerID string, before time.Time) (int64, error) {
	res, err := r.db.ExecContext(ctx,
		"DELETE FROM channels WHERE provider_id = ? AND last_seen_at < ?",
		providerID, before.Unix())
	if err != nil {
		return 0, fmt.Errorf("db.DeleteStale (provider=%s): %w", providerID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("db.DeleteStale (RowsAffected): %w", err)
	}
	return n, nil
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
			boolToInt(ch.IsAdult), now, now, now,
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

// buildChannelWhere arma la cláusula WHERE compartida por FindFiltered y
// CountFiltered. Vive en un solo sitio a propósito: si los dos construyeran el
// filtro por su cuenta, el contador acabaría discrepando de la lista y la app
// mostraría un número que no corresponde a lo que enseña.
func buildChannelWhere(f ports.ChannelFilter) (string, []any) {
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
		// Los category_id son compuestos ("Animation;Kids"), así que hay que
		// preguntar por pertenencia y no por igualdad: con "=" filtrar por Kids
		// devolvía 253 canales en vez de 337. Los ';' de guarda evitan que
		// "Kid" case con "Kids". Medido: 7ms de escaneo sobre 12k filas, no
		// compensa índice. LIKE ya es insensible a mayúsculas para ASCII, lo
		// que además resuelve news/News.
		where = append(where, "';' || category_id || ';' LIKE ?")
		args = append(args, "%;"+f.Category+";%")
	}
	if clause, qargs := qualityWhereClause(f.MinQuality); clause != "" {
		where = append(where, clause)
		args = append(args, qargs...)
	}
	if f.AliveOnly {
		// Visible mientras el canal no esté PROBADO muerto: basta con un stream
		// vivo, o uno que aún no haya agotado DeadFailThreshold fallos
		// consecutivos (lo que incluye los que nunca se han chequeado, con
		// fail_count 0). Mirar solo is_alive dejaría la histéresis en nada.
		//
		// Los canales SIN NINGÚN stream quedan fuera: son injugables
		// —/channels/stream devuelve 404— y aparecen cuando el proveedor
		// renombra un canal y deja huérfana la fila anterior.
		where = append(where, fmt.Sprintf(`EXISTS (
			SELECT 1 FROM streams s
			WHERE s.channel_id = channels.id
			  AND (s.is_alive = 1 OR s.fail_count < %d)
		)`, DeadFailThreshold))
	}

	if len(where) == 0 {
		return "1=1", args
	}
	return strings.Join(where, " AND "), args
}

// CountFiltered devuelve cuántos canales casan con el filtro, ignorando
// paginación. Lo consume la barra de filtros de la app: sin esto solo sabría
// cuántos canales lleva cargados, y el contador diría "500" con 12000 detrás.
func (r *SQLiteChannelRepository) CountFiltered(ctx context.Context, f ports.ChannelFilter) (int, error) {
	f = f.Normalize()
	whereSQL, args := buildChannelWhere(f)

	var n int
	if err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM channels WHERE "+whereSQL, args...).Scan(&n); err != nil {
		return 0, fmt.Errorf("db.CountFiltered: %w", err)
	}
	return n, nil
}

// FindFiltered aplica filtros combinados (país, categoría, calidad, texto) en una
// sola query dinámica. Es el método principal que usa el handler /channels.
func (r *SQLiteChannelRepository) FindFiltered(ctx context.Context, f ports.ChannelFilter) ([]domain.Channel, error) {
	f = f.Normalize()
	whereSQL, args := buildChannelWhere(f)

	// Salud agregada incrustada: la lista pinta el indicador de señal sin
	// N+1 requests a /channels/{id}/health.
	q := "SELECT" + channelColumns + `,
		EXISTS(SELECT 1 FROM streams s WHERE s.channel_id = channels.id AND s.last_checked IS NOT NULL) AS any_checked,
		EXISTS(SELECT 1 FROM streams s WHERE s.channel_id = channels.id AND s.is_alive = 1) AS any_alive,
		(SELECT MIN(s.latency_ms) FROM streams s WHERE s.channel_id = channels.id AND s.is_alive = 1) AS best_latency
		FROM channels WHERE ` + whereSQL +
		// Desempate por id: sin él, SQLite no garantiza un orden estable entre
		// nombres iguales, y sobre 12k canales con nombres repetidos una fila
		// puede repetirse entre páginas u omitirse.
		" ORDER BY name COLLATE NOCASE, id LIMIT ? OFFSET ?"
	args = append(args, f.Limit, f.Offset)

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("db.FindFiltered: %w", err)
	}
	defer rows.Close()
	return scanChannelsWithHealth(rows)
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

// scanChannelsWithHealth escanea filas de FindFiltered, que añaden las tres
// columnas de salud agregada (any_checked, any_alive, best_latency).
func scanChannelsWithHealth(rows *sql.Rows) ([]domain.Channel, error) {
	var channels []domain.Channel
	for rows.Next() {
		var (
			ch                                                    domain.Channel
			tvgID, logoURL, categoryID, languageCode, countryCode sql.NullString
			providerID                                            string
			isAdult                                               int
			createdAt, updatedAt                                  int64
			anyChecked, anyAlive                                  int
			bestLatency                                           sql.NullInt64
		)
		if err := rows.Scan(
			(*string)(&ch.ID), &tvgID, &ch.Name, &logoURL, &categoryID,
			&languageCode, &countryCode,
			&providerID, (*string)(&ch.ProviderType),
			&isAdult, &createdAt, &updatedAt,
			&anyChecked, &anyAlive, &bestLatency,
		); err != nil {
			return nil, fmt.Errorf("db.scanChannelsWithHealth: %w", err)
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
		if anyChecked == 1 {
			alive := anyAlive == 1
			ch.Alive = &alive
			ch.LatencyMs = bestLatency.Int64
		}
		channels = append(channels, ch)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db.scanChannelsWithHealth (rows.Err): %w", err)
	}
	return channels, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// Countries devuelve los países con canales y su recuento, ordenados por
// volumen. Lo consume el selector de país de la app.
func (r *SQLiteChannelRepository) Countries(ctx context.Context) ([]ports.Faceta, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT country_code, COUNT(*) FROM channels
		WHERE country_code IS NOT NULL AND country_code != ''
		GROUP BY country_code ORDER BY COUNT(*) DESC, country_code`)
	if err != nil {
		return nil, fmt.Errorf("db.Countries: %w", err)
	}
	defer rows.Close()

	var out []ports.Faceta
	for rows.Next() {
		var f ports.Faceta
		if err := rows.Scan(&f.Valor, &f.Count); err != nil {
			return nil, fmt.Errorf("db.Countries (scan): %w", err)
		}
		out = append(out, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db.Countries (rows.Err): %w", err)
	}
	return out, nil
}

// Categories descompone los category_id compuestos ("Animation;Kids") en sus
// categorías atómicas y las cuenta por separado: la taxonomía real son unas 30,
// no los 181 strings compuestos que hay en la columna. SQLite no tiene split,
// así que el troceo va en Go; son 12k filas agrupadas, es despreciable.
func (r *SQLiteChannelRepository) Categories(ctx context.Context) ([]ports.Faceta, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT category_id, COUNT(*) FROM channels
		WHERE category_id IS NOT NULL AND category_id != ''
		GROUP BY category_id`)
	if err != nil {
		return nil, fmt.Errorf("db.Categories: %w", err)
	}
	defer rows.Close()

	acum := map[string]int{}
	for rows.Next() {
		var compuesta string
		var n int
		if err := rows.Scan(&compuesta, &n); err != nil {
			return nil, fmt.Errorf("db.Categories (scan): %w", err)
		}
		for _, atomica := range strings.Split(compuesta, ";") {
			if atomica = strings.TrimSpace(atomica); atomica != "" {
				acum[atomica] += n
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db.Categories (rows.Err): %w", err)
	}

	out := make([]ports.Faceta, 0, len(acum))
	for v, n := range acum {
		out = append(out, ports.Faceta{Valor: v, Count: n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Valor < out[j].Valor
	})
	return out, nil
}
