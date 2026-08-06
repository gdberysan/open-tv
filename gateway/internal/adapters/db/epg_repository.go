package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/tu-org/iptv-ecosystem/gateway/internal/domain"
	"github.com/tu-org/iptv-ecosystem/gateway/internal/ports"
)

var _ ports.EPGRepository = (*SQLiteEPGRepository)(nil)

// SQLiteEPGRepository implementa ports.EPGRepository sobre SQLite.
type SQLiteEPGRepository struct {
	db *sql.DB
}

func NewEPGRepository(db *sql.DB) *SQLiteEPGRepository {
	return &SQLiteEPGRepository{db: db}
}

// El INSERT...SELECT expande cada entrada XMLTV (keyed por tvg-id) a todos
// los canales con ese tvg_id, satisfaciendo la FK epg_entries→channels y
// descartando en silencio programas de canales que no tenemos.
// ID determinista canal@inicio: un canal no puede tener dos programas con el
// mismo inicio, y el re-sync actualiza (upsert) en vez de duplicar.
const upsertEPGSQL = `
	INSERT INTO epg_entries (id, channel_id, title, description, start_at, end_at, created_at)
	SELECT c.id || '@' || CAST(? AS TEXT), c.id, ?, ?, ?, ?, ?
	FROM channels c
	WHERE c.tvg_id = ?
	ON CONFLICT(id) DO UPDATE SET
		title       = excluded.title,
		description = excluded.description,
		end_at      = excluded.end_at`

// SaveBatch persiste un lote en una transacción. El ChannelID de cada entrada
// es el tvg-id XMLTV emitido por el parser (ver ports.EPGRepository).
func (r *SQLiteEPGRepository) SaveBatch(ctx context.Context, entries []domain.EPGEntry) error {
	if len(entries) == 0 {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("db.EPG.SaveBatch (BeginTx): %w", err)
	}
	defer tx.Rollback() //nolint:errcheck — Rollback es no-op si Commit tuvo éxito

	stmt, err := tx.PrepareContext(ctx, upsertEPGSQL)
	if err != nil {
		return fmt.Errorf("db.EPG.SaveBatch (Prepare): %w", err)
	}
	defer stmt.Close()

	now := time.Now().Unix()
	for _, e := range entries {
		start := e.StartAt.Unix()
		if _, err := stmt.ExecContext(ctx,
			start, e.Title, nullStr(e.Description), start, e.EndAt.Unix(), now,
			string(e.ChannelID), // tvg-id
		); err != nil {
			return fmt.Errorf("db.EPG.SaveBatch (Exec tvg=%s): %w", e.ChannelID, err)
		}
	}

	return tx.Commit()
}

const epgColumns = ` channel_id, title, description, start_at, end_at `

func (r *SQLiteEPGRepository) FindByChannelAndWindow(ctx context.Context, channelID domain.ChannelID, from, to time.Time) ([]domain.EPGEntry, error) {
	// Solape de intervalos: el programa toca la ventana [from, to)
	rows, err := r.db.QueryContext(ctx,
		"SELECT"+epgColumns+`FROM epg_entries
		 WHERE channel_id = ? AND end_at > ? AND start_at < ?
		 ORDER BY start_at`,
		string(channelID), from.Unix(), to.Unix())
	if err != nil {
		return nil, fmt.Errorf("db.EPG.FindByChannelAndWindow (channel=%s): %w", channelID, err)
	}
	defer rows.Close()
	return scanEPGEntries(rows)
}

func (r *SQLiteEPGRepository) FindCurrentlyAiring(ctx context.Context, now time.Time) ([]domain.EPGEntry, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT"+epgColumns+`FROM epg_entries
		 WHERE start_at <= ? AND end_at > ?
		 ORDER BY channel_id`,
		now.Unix(), now.Unix())
	if err != nil {
		return nil, fmt.Errorf("db.EPG.FindCurrentlyAiring: %w", err)
	}
	defer rows.Close()
	return scanEPGEntries(rows)
}

func (r *SQLiteEPGRepository) DeleteEndedBefore(ctx context.Context, t time.Time) (int64, error) {
	res, err := r.db.ExecContext(ctx,
		"DELETE FROM epg_entries WHERE end_at < ?", t.Unix())
	if err != nil {
		return 0, fmt.Errorf("db.EPG.DeleteEndedBefore: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("db.EPG.DeleteEndedBefore (RowsAffected): %w", err)
	}
	return n, nil
}

func scanEPGEntries(rows *sql.Rows) ([]domain.EPGEntry, error) {
	var entries []domain.EPGEntry
	for rows.Next() {
		var (
			e              domain.EPGEntry
			desc           sql.NullString
			startAt, endAt int64
		)
		if err := rows.Scan(
			(*string)(&e.ChannelID), &e.Title, &desc, &startAt, &endAt,
		); err != nil {
			return nil, fmt.Errorf("db.scanEPGEntries: %w", err)
		}
		e.Description = desc.String
		e.StartAt = time.Unix(startAt, 0)
		e.EndAt = time.Unix(endAt, 0)
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db.scanEPGEntries (rows.Err): %w", err)
	}
	return entries, nil
}
