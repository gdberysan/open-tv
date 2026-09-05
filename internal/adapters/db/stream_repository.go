package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/gdberysan/open-tv/internal/domain"
	"github.com/gdberysan/open-tv/internal/ports"
)

var _ ports.StreamRepository = (*SQLiteStreamRepository)(nil)

// SQLiteStreamRepository implementa ports.StreamRepository sobre SQLite.
// Persiste las URLs de stream que antes solo vivían en el caché en memoria
// del provider, para que /channels/stream sobreviva reinicios sin re-sync.
type SQLiteStreamRepository struct {
	db *sql.DB
}

func NewStreamRepository(db *sql.DB) *SQLiteStreamRepository {
	return &SQLiteStreamRepository{db: db}
}

const streamColumns = ` id, channel_id, url, protocol, latency_ms, is_alive, last_checked `

// El upsert preserva latency_ms/is_alive/last_checked: son resultado del
// health-check, no del sync, y un re-sync no debe borrarlos. last_seen_at SÍ se
// refresca en cada sync: es la frontera que usa DeleteStale.
const upsertStreamSQL = `
	INSERT INTO streams (id, channel_id, url, protocol, referrer, user_agent,
	                     latency_ms, is_alive, last_checked, last_seen_at, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, NULL, 0, NULL, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		channel_id   = excluded.channel_id,
		url          = excluded.url,
		protocol     = excluded.protocol,
		referrer     = excluded.referrer,
		user_agent   = excluded.user_agent,
		last_seen_at = excluded.last_seen_at,
		updated_at   = excluded.updated_at`

func (r *SQLiteStreamRepository) Save(ctx context.Context, s domain.Stream) error {
	now := time.Now().Unix()
	if _, err := r.db.ExecContext(ctx, upsertStreamSQL,
		s.ID, string(s.ChannelID), s.URL, string(s.Protocol), s.Referrer, s.UserAgent, now, now, now,
	); err != nil {
		return fmt.Errorf("db.Stream.Save (id=%s): %w", s.ID, err)
	}
	return nil
}

// SaveBatch hace upsert de streams en una sola transacción con statement
// preparado, siguiendo el mismo patrón que ChannelRepository.SaveBatch.
func (r *SQLiteStreamRepository) SaveBatch(ctx context.Context, streams []domain.Stream) error {
	if len(streams) == 0 {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("db.Stream.SaveBatch (BeginTx): %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // Rollback es no-op si Commit tuvo éxito

	stmt, err := tx.PrepareContext(ctx, upsertStreamSQL)
	if err != nil {
		return fmt.Errorf("db.Stream.SaveBatch (Prepare): %w", err)
	}
	defer func() { _ = stmt.Close() }()

	now := time.Now().Unix()
	for _, s := range streams {
		if _, err := stmt.ExecContext(ctx,
			s.ID, string(s.ChannelID), s.URL, string(s.Protocol), s.Referrer, s.UserAgent, now, now, now,
		); err != nil {
			return fmt.Errorf("db.Stream.SaveBatch (Exec id=%s): %w", s.ID, err)
		}
	}

	return tx.Commit()
}

func (r *SQLiteStreamRepository) FindAll(ctx context.Context) ([]domain.Stream, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT"+streamColumns+"FROM streams ORDER BY id")
	if err != nil {
		return nil, fmt.Errorf("db.Stream.FindAll: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var streams []domain.Stream
	for rows.Next() {
		s, err := scanStream(rows)
		if err != nil {
			return nil, fmt.Errorf("db.Stream.FindAll (scan): %w", err)
		}
		streams = append(streams, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db.Stream.FindAll (rows.Err): %w", err)
	}
	return streams, nil
}

func (r *SQLiteStreamRepository) FindByChannelID(ctx context.Context, channelID domain.ChannelID) ([]domain.Stream, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT"+streamColumns+"FROM streams WHERE channel_id = ? ORDER BY id", string(channelID))
	if err != nil {
		return nil, fmt.Errorf("db.Stream.FindByChannelID (channel=%s): %w", channelID, err)
	}
	defer func() { _ = rows.Close() }()

	var streams []domain.Stream
	for rows.Next() {
		s, err := scanStream(rows)
		if err != nil {
			return nil, fmt.Errorf("db.Stream.FindByChannelID (scan): %w", err)
		}
		streams = append(streams, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db.Stream.FindByChannelID (rows.Err): %w", err)
	}
	return streams, nil
}

// FindBestByChannelID devuelve el stream vivo de menor latencia.
// Aprovecha el índice parcial idx_streams_latency (WHERE is_alive = 1).
func (r *SQLiteStreamRepository) FindBestByChannelID(ctx context.Context, channelID domain.ChannelID) (domain.Stream, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT"+streamColumns+`FROM streams
		 WHERE channel_id = ? AND is_alive = 1
		 ORDER BY latency_ms ASC LIMIT 1`, string(channelID))
	if err != nil {
		return domain.Stream{}, fmt.Errorf("db.Stream.FindBestByChannelID (channel=%s): %w", channelID, err)
	}
	defer func() { _ = rows.Close() }()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return domain.Stream{}, fmt.Errorf("db.Stream.FindBestByChannelID (rows.Err): %w", err)
		}
		return domain.Stream{}, fmt.Errorf("db.Stream.FindBestByChannelID: sin streams vivos para canal %s", channelID)
	}
	s, err := scanStream(rows)
	if err != nil {
		return domain.Stream{}, fmt.Errorf("db.Stream.FindBestByChannelID (scan): %w", err)
	}
	return s, nil
}

// FindMirrorsByChannelID devuelve todos los mirrors del canal ordenados: vivos
// primero (ORDER BY is_alive DESC), dentro de vivos por latencia ascendente
// (NULLS LAST para que un vivo sin latencia medida no encabece); los muertos
// caen al final por el is_alive DESC.
func (r *SQLiteStreamRepository) FindMirrorsByChannelID(ctx context.Context, channelID domain.ChannelID) ([]ports.MirrorHealth, error) {
	const q = `SELECT url, is_alive, COALESCE(latency_ms, 0), web_ok
	           FROM streams WHERE channel_id = ?
	           ORDER BY is_alive DESC,
	                    CASE WHEN latency_ms IS NULL THEN 1 ELSE 0 END,
	                    latency_ms ASC`
	rows, err := r.db.QueryContext(ctx, q, string(channelID))
	if err != nil {
		return nil, fmt.Errorf("db.Stream.FindMirrorsByChannelID (query %s): %w", channelID, err)
	}
	defer func() { _ = rows.Close() }()

	var mirrors []ports.MirrorHealth
	for rows.Next() {
		var (
			m       ports.MirrorHealth
			aliveIn int
			webOK   sql.NullInt64
		)
		if err := rows.Scan(&m.URL, &aliveIn, &m.LatencyMs, &webOK); err != nil {
			return nil, fmt.Errorf("db.Stream.FindMirrorsByChannelID (scan): %w", err)
		}
		m.IsAlive = aliveIn == 1
		switch {
		case !webOK.Valid:
			m.WebOK = domain.WebUnknown
		case webOK.Int64 == 1:
			m.WebOK = domain.WebOK
		default:
			m.WebOK = domain.WebNo
		}
		mirrors = append(mirrors, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db.Stream.FindMirrorsByChannelID (rows.Err): %w", err)
	}
	return mirrors, nil
}

// DeadFailThreshold es el número de chequeos fallidos consecutivos necesarios
// para dar un stream por muerto. Un solo HEAD fallido no basta: como el filtro
// AliveOnly está activo por defecto, un blip de red ocultaría el canal hasta la
// siguiente pasada del worker, una hora después.
const DeadFailThreshold int64 = 3

func (r *SQLiteStreamRepository) MarkAlive(ctx context.Context, streamID string, latencyMs int64) error {
	now := time.Now().Unix()
	if _, err := r.db.ExecContext(ctx,
		`UPDATE streams
		 SET is_alive = 1, fail_count = 0, latency_ms = ?, last_checked = ?, updated_at = ?
		 WHERE id = ?`,
		latencyMs, now, now, streamID,
	); err != nil {
		return fmt.Errorf("db.Stream.MarkAlive (id=%s): %w", streamID, err)
	}
	return nil
}

// MarkDead cuenta el fallo y solo apaga is_alive al alcanzar DeadFailThreshold.
// El incremento y la comparación van en la misma sentencia para que no haya
// lectura-modificación-escritura ni carrera entre pasadas.
func (r *SQLiteStreamRepository) MarkDead(ctx context.Context, streamID string) error {
	now := time.Now().Unix()
	if _, err := r.db.ExecContext(ctx,
		`UPDATE streams
		 SET fail_count   = fail_count + 1,
		     is_alive     = CASE WHEN fail_count + 1 >= ? THEN 0 ELSE is_alive END,
		     last_checked = ?,
		     updated_at   = ?
		 WHERE id = ?`,
		DeadFailThreshold, now, now, streamID,
	); err != nil {
		return fmt.Errorf("db.Stream.MarkDead (id=%s): %w", streamID, err)
	}
	return nil
}

func scanStream(rows *sql.Rows) (domain.Stream, error) {
	var (
		s           domain.Stream
		latencyMs   sql.NullInt64
		isAlive     int
		lastChecked sql.NullInt64
	)
	if err := rows.Scan(
		&s.ID, (*string)(&s.ChannelID), &s.URL, (*string)(&s.Protocol),
		&latencyMs, &isAlive, &lastChecked,
	); err != nil {
		return domain.Stream{}, err
	}
	s.LatencyMs = latencyMs.Int64
	s.IsAlive = isAlive == 1
	if lastChecked.Valid {
		s.LastChecked = time.Unix(lastChecked.Int64, 0)
	}
	return s, nil
}

// MarkBatch aplica todos los resultados del health-check en una transacción.
// Medido sobre una copia de la DB real: 2000 UPDATEs sueltos tardan ~90ms; los
// mismos 2000 dentro de una transacción, ~8.6ms. Con MaxOpenConns(1) ese
// tiempo de escritura es tiempo en el que la API no puede leer.
func (r *SQLiteStreamRepository) MarkBatch(ctx context.Context, resultados []ports.StreamHealth) error {
	if len(resultados) == 0 {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("db.Stream.MarkBatch (BeginTx): %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // Rollback es no-op si Commit tuvo éxito

	stmtVivo, err := tx.PrepareContext(ctx,
		`UPDATE streams
		 SET is_alive = 1, fail_count = 0, latency_ms = ?, last_checked = ?, updated_at = ?,
		     web_ok = COALESCE(?, web_ok)
		 WHERE id = ?`)
	if err != nil {
		return fmt.Errorf("db.Stream.MarkBatch (Prepare vivo): %w", err)
	}
	defer func() { _ = stmtVivo.Close() }()

	stmtMuerto, err := tx.PrepareContext(ctx,
		`UPDATE streams
		 SET fail_count   = fail_count + 1,
		     is_alive     = CASE WHEN fail_count + 1 >= ? THEN 0 ELSE is_alive END,
		     last_checked = ?,
		     updated_at   = ?,
		     web_ok       = COALESCE(?, web_ok)
		 WHERE id = ?`)
	if err != nil {
		return fmt.Errorf("db.Stream.MarkBatch (Prepare muerto): %w", err)
	}
	defer func() { _ = stmtMuerto.Close() }()

	now := time.Now().Unix()
	for _, res := range resultados {
		if res.IsAlive {
			_, err = stmtVivo.ExecContext(ctx, res.LatencyMs, now, now, argWebOK(res.Web), res.StreamID)
		} else {
			_, err = stmtMuerto.ExecContext(ctx, DeadFailThreshold, now, now, argWebOK(res.Web), res.StreamID)
		}
		if err != nil {
			return fmt.Errorf("db.Stream.MarkBatch (Exec id=%s): %w", res.StreamID, err)
		}
	}

	return tx.Commit()
}

// argWebOK traduce el veredicto al argumento del COALESCE: nil para
// "no se sabe", que deja intacto lo que ya hubiera en la columna.
func argWebOK(v domain.WebSupport) any {
	switch v {
	case domain.WebOK:
		return int64(1)
	case domain.WebNo:
		return int64(0)
	default:
		return nil
	}
}

// DeleteStale borra los streams de la fuente que no aparecieron en este sync.
// streams no tiene provider_id, así que el scoping va por el canal — igual de
// estricto: la poda de una fuente nunca toca los streams de otra.
func (r *SQLiteStreamRepository) DeleteStale(ctx context.Context, providerID string, before time.Time) (int64, error) {
	res, err := r.db.ExecContext(ctx, `
		DELETE FROM streams
		WHERE last_seen_at < ?
		  AND channel_id IN (SELECT id FROM channels WHERE provider_id = ?)`,
		before.Unix(), providerID)
	if err != nil {
		return 0, fmt.Errorf("db.Stream.DeleteStale (provider=%s): %w", providerID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("db.Stream.DeleteStale (RowsAffected): %w", err)
	}
	return n, nil
}

// CabecerasPorURL busca las cabeceras del stream con esa URL. Una URL que no
// está en el catálogo devuelve vacías sin error: significa "usa las de siempre".
func (r *SQLiteStreamRepository) CabecerasPorURL(ctx context.Context, url string) (string, string, error) {
	var referrer, userAgent string
	err := r.db.QueryRowContext(ctx,
		"SELECT referrer, user_agent FROM streams WHERE url = ? LIMIT 1", url).
		Scan(&referrer, &userAgent)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", nil
	}
	if err != nil {
		return "", "", fmt.Errorf("db.Stream.CabecerasPorURL: %w", err)
	}
	return referrer, userAgent, nil
}
