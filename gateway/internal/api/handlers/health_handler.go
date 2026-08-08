package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"time"
)

// maxSyncAge es la edad a partir de la cual el catálogo se considera rancio.
// Dos intervalos de sync (12h) más margen: por debajo de eso, un fallo aislado
// con reintentos todavía es operación normal.
const maxSyncAge = 26 * time.Hour

// SyncStatus expone lo que /health necesita saber del syncer sin acoplarse al
// tipo concreto.
type SyncStatus interface {
	LastSuccess() time.Time
}

// HealthHandler responde /health con el estado real: si la DB contesta y si el
// catálogo se ha sincronizado hace poco. El literal {"status":"ok"} anterior no
// distinguía un gateway sano de uno cuyo syncer llevaba días fallando o cuyo
// fichero de DB había desaparecido.
type HealthHandler struct {
	db     *sql.DB
	syncer SyncStatus
}

func NewHealthHandler(db *sql.DB, syncer SyncStatus) *HealthHandler {
	return &HealthHandler{db: db, syncer: syncer}
}

func (h *HealthHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	res := map[string]any{"status": "ok", "db": "ok"}
	httpStatus := http.StatusOK

	if err := h.db.PingContext(ctx); err != nil {
		res["db"] = "error"
		res["status"] = "degraded"
		httpStatus = http.StatusServiceUnavailable
	}

	if h.syncer != nil {
		last := h.syncer.LastSuccess()
		if last.IsZero() {
			res["last_sync"] = nil
			res["sync_age_seconds"] = nil
		} else {
			edad := time.Since(last)
			res["last_sync"] = last.Format(time.RFC3339)
			res["sync_age_seconds"] = int64(edad.Seconds())
			if edad > maxSyncAge {
				res["status"] = "degraded"
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)
	_ = json.NewEncoder(w).Encode(res)
}
