package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
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

// Las cabeceras van en el SELECT: sin ellas domain.Stream.Referrer/UserAgent
// volvían SIEMPRE vacíos y el health-check —que lee por FindAll— chequeaba
// pelados los streams cuyo origen exige Referer/User-Agent, se comía un 403 y
// acababa marcándolos muertos.
const streamColumns = ` id, channel_id, url, protocol, referrer, user_agent, latency_ms, is_alive, last_checked, codec_checked_at `

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
	const q = `SELECT url, is_alive, COALESCE(latency_ms, 0), web_ok, codec_ok, codecs
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
			codecOK sql.NullInt64
		)
		if err := rows.Scan(&m.URL, &aliveIn, &m.LatencyMs, &webOK, &codecOK, &m.Codecs); err != nil {
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
		switch {
		case !codecOK.Valid:
			m.Codec = domain.CodecUnknown
		case codecOK.Int64 == 1:
			m.Codec = domain.CodecOK
		default:
			m.Codec = domain.CodecNo
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
		s              domain.Stream
		latencyMs      sql.NullInt64
		isAlive        int
		lastChecked    sql.NullInt64
		codecCheckedAt int64
	)
	if err := rows.Scan(
		&s.ID, (*string)(&s.ChannelID), &s.URL, (*string)(&s.Protocol),
		&s.Referrer, &s.UserAgent,
		&latencyMs, &isAlive, &lastChecked, &codecCheckedAt,
	); err != nil {
		return domain.Stream{}, err
	}
	s.LatencyMs = latencyMs.Int64
	s.IsAlive = isAlive == 1
	if lastChecked.Valid {
		s.LastChecked = time.Unix(lastChecked.Int64, 0)
	}
	if codecCheckedAt != 0 {
		s.CodecCheckedAt = time.Unix(codecCheckedAt, 0)
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
		     web_ok = COALESCE(?, web_ok),
		     codec_ok = COALESCE(?, codec_ok),
		     codecs = COALESCE(?, codecs),
		     codec_checked_at = CASE WHEN ? = 1 THEN ? ELSE codec_checked_at END
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
			_, err = stmtVivo.ExecContext(ctx, res.LatencyMs, now, now, argWebOK(res.Web),
				argCodecOK(res.Codec), argCodecs(res), boolAInt(res.CodecSondeado), now, res.StreamID)
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

// argCodecOK y argCodecs: nil para "no se sabe", que el COALESCE deja
// intacto. codecs solo se escribe con un veredicto real.
func argCodecOK(v domain.CodecSupport) any {
	switch v {
	case domain.CodecOK:
		return int64(1)
	case domain.CodecNo:
		return int64(0)
	default:
		return nil
	}
}

func argCodecs(res ports.StreamHealth) any {
	if res.Codec == domain.CodecUnknown {
		return nil
	}
	return res.Codecs
}

func boolAInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
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

// CabecerasPorURL busca las cabeceras del stream con esa URL. Si la URL
// exacta no está, cae a cualquier fila del MISMO ORIGEN (scheme+host) que sí
// declare cabeceras: esto cubre los segmentos y las URIs de EXT-X-KEY/
// EXT-X-MAP que ReescribirManifiesto reescribe, que nunca son una fila propia
// de streams —solo el manifiesto de nivel superior lo es— pero a los que la
// misma protección de hotlink/agent-filter del origen les aplica igual. El
// exacto SIEMPRE gana sobre el origen (incluso si sus cabeceras están
// vacías): una URL que sí está en el catálogo no debe heredar nada ajeno.
// Ni exacto ni origen: cadenas vacías sin error, que es "usa las de siempre".
func (r *SQLiteStreamRepository) CabecerasPorURL(ctx context.Context, urlCruda string) (string, string, error) {
	var referrer, userAgent string
	err := r.db.QueryRowContext(ctx,
		"SELECT referrer, user_agent FROM streams WHERE url = ? LIMIT 1", urlCruda).
		Scan(&referrer, &userAgent)
	if err == nil {
		return referrer, userAgent, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", "", fmt.Errorf("db.Stream.CabecerasPorURL: %w", err)
	}

	prefijo, tope, ok := origenPrefijo(urlCruda)
	if !ok {
		return "", "", nil
	}
	err = r.db.QueryRowContext(ctx, `
		SELECT referrer, user_agent FROM streams
		WHERE url >= ? AND url < ?
		  AND (referrer <> '' OR user_agent <> '')
		LIMIT 1`, prefijo, tope).
		Scan(&referrer, &userAgent)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", nil
	}
	if err != nil {
		return "", "", fmt.Errorf("db.Stream.CabecerasPorURL (fallback por origen): %w", err)
	}
	return referrer, userAgent, nil
}

// origenPrefijo calcula el rango [prefijo, tope) que idx_streams_url puede
// recorrer como range scan (nunca un LIKE con comodín inicial, que fuerza un
// escaneo completo) para encontrar cualquier fila cuya url empiece por
// "scheme://host/" — el mismo origen que urlCruda, sea o no su URL exacta.
//
// tope es prefijo con su último byte ('/' = 0x2F) incrementado a '0' (0x30).
// Cualquier fila cuya url EMPIECE por prefijo cae en [prefijo, tope): en la
// comparación de bytes diverge justo en esa posición con un valor menor que
// '0'. Y un host que solo comparta el prefijo como subcadena sin el separador
// "/" (p.ej. "con.example.evil.com" frente a "con.example") queda FUERA del
// rango: su siguiente byte ahí no es '/', así que la comparación ya se
// decide antes de llegar a esa posición, en un sentido o en otro según sea
// mayor o menor que '/'.
func origenPrefijo(urlCruda string) (prefijo, tope string, ok bool) {
	u, err := url.Parse(urlCruda)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", "", false
	}
	prefijo = u.Scheme + "://" + u.Host + "/"
	tope = prefijo[:len(prefijo)-1] + "0"
	return prefijo, tope, true
}
