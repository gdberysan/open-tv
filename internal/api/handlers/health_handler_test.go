package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/gdberysan/open-tv/internal/adapters/db"
)

type fakeSyncStatus struct{ last time.Time }

func (f fakeSyncStatus) LastSuccess() time.Time { return f.last }

func TestHealthReportaEdadDelSync(t *testing.T) {
	sqlDB, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer sqlDB.Close()

	h := NewHealthHandler(sqlDB, fakeSyncStatus{last: time.Now().Add(-2 * time.Hour)}, Info{})
	rec := httptest.NewRecorder()
	h.Get(rec, httptest.NewRequest(http.MethodGet, "/health", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, quiero 200", rec.Code)
	}
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("respuesta no es JSON: %v", err)
	}
	if got["status"] != "ok" {
		t.Errorf("status = %v, quiero ok", got["status"])
	}
	if got["db"] != "ok" {
		t.Errorf("db = %v, quiero ok", got["db"])
	}
	edad, ok := got["sync_age_seconds"].(float64)
	if !ok || edad < 7000 || edad > 7400 {
		t.Errorf("sync_age_seconds = %v, quiero ~7200", got["sync_age_seconds"])
	}
}

// Un syncer que lleva días fallando no puede reportar salud perfecta.
func TestHealthDegradadoSiElSyncEsMuyViejo(t *testing.T) {
	sqlDB, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer sqlDB.Close()

	h := NewHealthHandler(sqlDB, fakeSyncStatus{last: time.Now().Add(-72 * time.Hour)}, Info{})
	rec := httptest.NewRecorder()
	h.Get(rec, httptest.NewRequest(http.MethodGet, "/health", nil))

	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("respuesta no es JSON: %v", err)
	}
	if got["status"] != "degraded" {
		t.Errorf("status = %v, quiero degraded con un sync de 72h", got["status"])
	}
}

// Una DB caída debe reflejarse, no ocultarse tras un literal.
func TestHealthDetectaDBCaida(t *testing.T) {
	sqlDB, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	sqlDB.Close()

	h := NewHealthHandler(sqlDB, fakeSyncStatus{last: time.Now()}, Info{})
	rec := httptest.NewRecorder()
	h.Get(rec, httptest.NewRequest(http.MethodGet, "/health", nil))

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status HTTP = %d, quiero 503 con la DB caída", rec.Code)
	}
}

// Antes del primer sync no hay edad que reportar, pero el gateway está sano.
func TestHealthSinSyncPrevio(t *testing.T) {
	sqlDB, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer sqlDB.Close()

	h := NewHealthHandler(sqlDB, fakeSyncStatus{}, Info{})
	rec := httptest.NewRecorder()
	h.Get(rec, httptest.NewRequest(http.MethodGet, "/health", nil))

	var got map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if got["status"] != "ok" {
		t.Errorf("status = %v, quiero ok", got["status"])
	}
	if got["last_sync"] != nil {
		t.Errorf("last_sync = %v, quiero null", got["last_sync"])
	}
}

func TestHealthPublicaLaInfoDelBinario(t *testing.T) {
	sqlDB, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer sqlDB.Close()

	h := NewHealthHandler(sqlDB, fakeSyncStatus{}, Info{
		Version: "1.2.3", WebUI: true, ProxyEnabled: true,
	})
	rec := httptest.NewRecorder()
	h.Get(rec, httptest.NewRequest(http.MethodGet, "/health", nil))

	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	for k, quiero := range map[string]any{"version": "1.2.3", "web_ui": true, "proxy_enabled": true} {
		if got[k] != quiero {
			t.Errorf("%s = %v, quiero %v", k, got[k], quiero)
		}
	}
}

// Sin versión inyectada por el linker (go run, go test) la respuesta dice
// "dev" y no una cadena vacía que parezca un bug.
func TestHealthVersionPorDefecto(t *testing.T) {
	sqlDB, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer sqlDB.Close()

	h := NewHealthHandler(sqlDB, fakeSyncStatus{}, Info{})
	rec := httptest.NewRecorder()
	h.Get(rec, httptest.NewRequest(http.MethodGet, "/health", nil))

	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got["version"] != "dev" {
		t.Errorf("version = %v, quiero \"dev\"", got["version"])
	}
}
