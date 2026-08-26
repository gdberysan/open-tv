package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/gdberysan/open-tv/internal/domain"
	"github.com/gdberysan/open-tv/internal/ports"
)

var _ ports.EPGRepository = (*SQLiteEPGRepository)(nil)

// SQLiteEPGRepository implementa ports.EPGRepository sobre SQLite. La guía
// vive en epg_programmes, clavada por (provider_id, channel_id XMLTV); el
// join hacia channels SIEMPRE lleva provider_id además de tvg_id — ver
// schema.sql y el comentario de EPGRepository.
type SQLiteEPGRepository struct {
	db *sql.DB
}

func NewEPGRepository(db *sql.DB) *SQLiteEPGRepository {
	return &SQLiteEPGRepository{db: db}
}

const upsertEPGSQL = `
	INSERT INTO epg_programmes (provider_id, channel_id, start_utc, stop_utc, title, sub_title, description)
	VALUES (?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(provider_id, channel_id, start_utc) DO UPDATE SET
		stop_utc    = excluded.stop_utc,
		title       = excluded.title,
		sub_title   = excluded.sub_title,
		description = excluded.description`

// ReemplazarVentana borra TODA la guía de ese provider y escribe la ventana
// nueva, en una sola transacción: un sync no acumula sobre el anterior, lo
// reemplaza. El ON CONFLICT del INSERT es una defensa extra por si el XMLTV
// de origen repite una fila (provider_id, channel_id, start_utc) dentro del
// mismo lote — no debería pasar tras el DELETE previo, pero así no revienta.
func (r *SQLiteEPGRepository) ReemplazarVentana(ctx context.Context, providerID string, programas []domain.Programa) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("db.EPG.ReemplazarVentana (BeginTx): %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // Rollback es no-op si Commit tuvo éxito

	if _, err := tx.ExecContext(ctx, "DELETE FROM epg_programmes WHERE provider_id = ?", providerID); err != nil {
		return fmt.Errorf("db.EPG.ReemplazarVentana (DELETE provider=%s): %w", providerID, err)
	}

	if len(programas) > 0 {
		stmt, err := tx.PrepareContext(ctx, upsertEPGSQL)
		if err != nil {
			return fmt.Errorf("db.EPG.ReemplazarVentana (Prepare): %w", err)
		}
		defer func() { _ = stmt.Close() }()

		for _, p := range programas {
			if _, err := stmt.ExecContext(ctx,
				providerID, p.ChannelID, p.InicioUTC, p.FinUTC, p.Titulo,
				nullStr(p.Subtitulo), nullStr(p.Descripcion),
			); err != nil {
				return fmt.Errorf("db.EPG.ReemplazarVentana (Exec provider=%s channel=%s start=%d): %w",
					providerID, p.ChannelID, p.InicioUTC, err)
			}
		}
	}

	return tx.Commit()
}

// Podar borra los programas ya terminados de CUALQUIER provider. Es
// mantenimiento periódico (retención de la ventana), no depende del sync de
// una fuente concreta.
func (r *SQLiteEPGRepository) Podar(ctx context.Context, antesDe int64) error {
	if _, err := r.db.ExecContext(ctx, "DELETE FROM epg_programmes WHERE stop_utc < ?", antesDe); err != nil {
		return fmt.Errorf("db.EPG.Podar (antesDe=%d): %w", antesDe, err)
	}
	return nil
}

// AhoraDespuesPorCanales resuelve, en dos consultas (nunca N+1 aunque el lote
// tenga cientos de canales), el programa en curso y el inmediato siguiente
// de cada canal pedido:
//
//  1. Qué canales del lote TIENEN tvg_id no vacío. Solo esos entran en el
//     mapa de salida (con AhoraDespues{} por defecto); un tvg_id vacío (o el
//     canal ni siquiera existe en `channels`) significa que esa fuente no
//     puede tener guía para él, así que el canal queda AUSENTE — el handler
//     lo distingue de "hay guía pero ahora mismo no hay nada" mirando si la
//     clave está en el mapa.
//  2. Una única consulta con CTE que une channels⨝epg_programmes por
//     (provider_id, tvg_id) —el aislamiento por fuente— y clasifica cada fila
//     candidata (stop_utc > ahora, o sea "no terminado todavía") en 'ahora'
//     (start_utc <= ahora < stop_utc, con el filtro exterior stop_utc>ahora
//     ya garantizando el "< stop_utc") o 'siguiente' (start_utc > ahora); un
//     ROW_NUMBER() por (canal, categoría) se queda con la fila de menor
//     start_utc en cada una. Así, un programa que termina justo en `ahora`
//     nunca cuenta como "ahora" (stop_utc > ahora falla), y uno que empieza
//     justo ahí sí cuenta.
func (r *SQLiteEPGRepository) AhoraDespuesPorCanales(ctx context.Context, channelIDs []string, ahora int64) (map[string]domain.AhoraDespues, error) {
	out := make(map[string]domain.AhoraDespues)
	if len(channelIDs) == 0 {
		return out, nil
	}

	marcas := strings.Repeat("?,", len(channelIDs)-1) + "?"
	idArgs := make([]any, len(channelIDs))
	for i, id := range channelIDs {
		idArgs[i] = id
	}

	// Paso 1: qué channelIDs pedidos tienen guía posible (tvg_id no vacío).
	// #nosec G202 -- marcas es solo una lista fija de "?" (uno por channelID); todos los valores van parametrizados en idArgs, nunca concatenados
	rowsTvg, err := r.db.QueryContext(ctx,
		"SELECT id FROM channels WHERE id IN ("+marcas+") AND tvg_id IS NOT NULL AND tvg_id != ''", idArgs...)
	if err != nil {
		return nil, fmt.Errorf("db.EPG.AhoraDespuesPorCanales (canales con tvg_id): %w", err)
	}
	for rowsTvg.Next() {
		var id string
		if err := rowsTvg.Scan(&id); err != nil {
			_ = rowsTvg.Close()
			return nil, fmt.Errorf("db.EPG.AhoraDespuesPorCanales (scan id): %w", err)
		}
		out[id] = domain.AhoraDespues{}
	}
	if err := rowsTvg.Err(); err != nil {
		_ = rowsTvg.Close()
		return nil, fmt.Errorf("db.EPG.AhoraDespuesPorCanales (rows.Err canales): %w", err)
	}
	_ = rowsTvg.Close()

	if len(out) == 0 {
		return out, nil
	}

	// Paso 2: los propios programas, en una sola consulta para todo el lote.
	// #nosec G202 -- marcas es solo una lista fija de "?" (uno por channelID); todos los valores van parametrizados en qargs, nunca concatenados
	q := `
		WITH candidatos AS (
			SELECT c.id AS chid, e.channel_id AS prog_channel_id,
			       e.start_utc, e.stop_utc, e.title, e.sub_title, e.description,
			       CASE WHEN e.start_utc <= ? AND e.stop_utc > ? THEN 1 ELSE 0 END AS es_ahora
			FROM channels c
			JOIN epg_programmes e ON e.provider_id = c.provider_id AND e.channel_id = c.tvg_id
			WHERE c.id IN (` + marcas + `)
			  AND c.tvg_id IS NOT NULL AND c.tvg_id != ''
			  AND e.stop_utc > ?
		),
		rankeados AS (
			SELECT *, ROW_NUMBER() OVER (PARTITION BY chid, es_ahora ORDER BY start_utc ASC) AS rn
			FROM candidatos
		)
		SELECT chid, prog_channel_id, es_ahora, start_utc, stop_utc, title, sub_title, description
		FROM rankeados
		WHERE rn = 1`

	qargs := make([]any, 0, len(idArgs)+3)
	qargs = append(qargs, ahora, ahora)
	qargs = append(qargs, idArgs...)
	qargs = append(qargs, ahora)

	rows, err := r.db.QueryContext(ctx, q, qargs...)
	if err != nil {
		return nil, fmt.Errorf("db.EPG.AhoraDespuesPorCanales (query): %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var (
			chid, progChannelID    string
			esAhora                int
			startUTC, stopUTC      int64
			title                  string
			subTitulo, descripcion sql.NullString
		)
		if err := rows.Scan(&chid, &progChannelID, &esAhora, &startUTC, &stopUTC, &title, &subTitulo, &descripcion); err != nil {
			return nil, fmt.Errorf("db.EPG.AhoraDespuesPorCanales (scan): %w", err)
		}
		p := &domain.Programa{
			ChannelID:   progChannelID,
			InicioUTC:   startUTC,
			FinUTC:      stopUTC,
			Titulo:      title,
			Subtitulo:   subTitulo.String,
			Descripcion: descripcion.String,
		}
		ad := out[chid]
		if esAhora == 1 {
			ad.Ahora = p
		} else {
			ad.Siguiente = p
		}
		out[chid] = ad
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db.EPG.AhoraDespuesPorCanales (rows.Err): %w", err)
	}

	return out, nil
}

// ProximosDeCanal devuelve el programa en curso (si lo hay) seguido de los
// próximos `limite` programas de un canal, ordenados por inicio. El join
// hacia epg_programmes va por (provider_id, tvg_id) del canal pedido, así que
// resuelve solo la guía de SU fuente.
func (r *SQLiteEPGRepository) ProximosDeCanal(ctx context.Context, channelID string, ahora int64, limite int) ([]domain.Programa, error) {
	if limite < 0 {
		limite = 0
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT e.channel_id, e.start_utc, e.stop_utc, e.title, e.sub_title, e.description
		FROM epg_programmes e
		JOIN channels c ON c.provider_id = e.provider_id AND c.tvg_id = e.channel_id
		WHERE c.id = ? AND e.stop_utc > ?
		ORDER BY e.start_utc ASC
		LIMIT ?`, channelID, ahora, limite+1)
	if err != nil {
		return nil, fmt.Errorf("db.EPG.ProximosDeCanal (channel=%s): %w", channelID, err)
	}
	defer func() { _ = rows.Close() }()

	var out []domain.Programa
	for rows.Next() {
		var (
			p                      domain.Programa
			subTitulo, descripcion sql.NullString
		)
		if err := rows.Scan(&p.ChannelID, &p.InicioUTC, &p.FinUTC, &p.Titulo, &subTitulo, &descripcion); err != nil {
			return nil, fmt.Errorf("db.EPG.ProximosDeCanal (scan): %w", err)
		}
		p.Subtitulo = subTitulo.String
		p.Descripcion = descripcion.String
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db.EPG.ProximosDeCanal (rows.Err): %w", err)
	}
	return out, nil
}
