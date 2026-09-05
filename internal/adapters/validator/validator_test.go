package validator

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestChecker_Check(t *testing.T) {
	tests := []struct {
		name         string
		headStatus   int
		getStatus    int
		urlSuffix    string
		wantAlive    bool
		wantProtocol string
	}{
		{
			name:         "HEAD returns 200",
			headStatus:   http.StatusOK,
			getStatus:    http.StatusOK,
			urlSuffix:    "/stream.m3u8",
			wantAlive:    true,
			wantProtocol: "HLS",
		},
		{
			name:         "HEAD returns 405, GET returns 200",
			headStatus:   http.StatusMethodNotAllowed,
			getStatus:    http.StatusOK,
			urlSuffix:    "/stream.mpd",
			wantAlive:    true,
			wantProtocol: "DASH",
		},
		{
			name:         "HEAD returns 404, GET returns 404",
			headStatus:   http.StatusNotFound,
			getStatus:    http.StatusNotFound,
			urlSuffix:    "/notfound.m3u8",
			wantAlive:    false,
			wantProtocol: "HLS",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodHead {
					w.WriteHeader(tt.headStatus)
					return
				}
				if r.Method == http.MethodGet {
					w.WriteHeader(tt.getStatus)
					return
				}
			}))
			defer server.Close()

			checker := NewChecker(server.Client(), 2*time.Second)
			res := checker.Check(context.Background(), server.URL+tt.urlSuffix)

			if res.IsAlive != tt.wantAlive {
				t.Errorf("IsAlive = %v, want %v", res.IsAlive, tt.wantAlive)
			}
			if res.Protocol != tt.wantProtocol {
				t.Errorf("Protocol = %v, want %v", res.Protocol, tt.wantProtocol)
			}
			if res.Error != nil {
				t.Errorf("Unexpected error: %v", res.Error)
			}
		})
	}
}

func TestValidator_Start(t *testing.T) {
	// Servidor mock que responde siempre OK
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond) // Simulamos algo de latencia
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := Config{MaxWorkers: 5, Timeout: 1 * time.Second}
	checker := NewChecker(server.Client(), cfg.Timeout)
	val := NewValidator(cfg, checker)

	ctx := context.Background()
	tareas := make(chan TareaCheck, 10)

	// Llenamos el canal con tareas
	for i := 0; i < 10; i++ {
		tareas <- TareaCheck{URL: server.URL + "/stream.m3u8"}
	}
	close(tareas)

	results := val.Start(ctx, tareas)

	var count int32
	for res := range results {
		if !res.IsAlive {
			t.Errorf("Expected IsAlive true, got false for %s", res.URL)
		}
		atomic.AddInt32(&count, 1)
	}

	if count != 10 {
		t.Errorf("Expected 10 results, got %d", count)
	}
}

func TestChecker_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Timeout muy corto para provocar fallo
	checker := NewChecker(server.Client(), 10*time.Millisecond)
	res := checker.Check(context.Background(), server.URL+"/stream.m3u8")

	if res.IsAlive {
		t.Error("Expected IsAlive false due to timeout")
	}
	if res.Error == nil {
		t.Error("Expected context deadline exceeded error, got nil")
	}
}
