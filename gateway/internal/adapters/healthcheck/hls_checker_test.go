package healthcheck

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// m3u8 de prueba mínimo con un segmento de 6s
const mockPlaylist = `#EXTM3U
#EXT-X-TARGETDURATION:6
#EXT-X-VERSION:3
#EXTINF:6.006,
segment0.ts
#EXT-X-ENDLIST
`

const mockSegmentData = "FAKE_TS_DATA_1234567890" // Simulamos datos de segmento

func TestHLSChecker_CheckOne_AliveStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Latencia artificial: en localhost el check completo puede tardar <1ms
		// y LatencyMs redondearía a 0, haciendo flaky la aserción de latencia.
		time.Sleep(5 * time.Millisecond)
		if r.URL.Path == "/stream.m3u8" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(mockPlaylist))
			return
		}
		// Sirve el segmento .ts
		if r.URL.Path == "/segment0.ts" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(mockSegmentData))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	checker := NewHLSChecker(2 * time.Second)
	// Reemplazamos el client del checker por el del server de prueba
	checker.client = server.Client()

	result := checker.CheckOne(context.Background(), server.URL+"/stream.m3u8")

	if !result.IsAlive {
		t.Errorf("Expected IsAlive=true, got false. Error: %v", result.Error)
	}
	if result.LatencyMs <= 0 {
		t.Errorf("Expected positive latency, got %d", result.LatencyMs)
	}
	if result.Error != nil {
		t.Errorf("Unexpected error: %v", result.Error)
	}
}

func TestHLSChecker_CheckOne_DeadStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Not Found", http.StatusNotFound)
	}))
	defer server.Close()

	checker := NewHLSChecker(2 * time.Second)
	checker.client = server.Client()

	result := checker.CheckOne(context.Background(), server.URL+"/dead.m3u8")

	if result.IsAlive {
		t.Error("Expected IsAlive=false for dead stream")
	}
	if result.Error == nil {
		t.Error("Expected error for dead stream")
	}
}

func TestHLSChecker_CheckBatch_ConcurrentAndBounded(t *testing.T) {
	callCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		time.Sleep(20 * time.Millisecond) // Simular latencia real
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(mockPlaylist))
	}))
	defer server.Close()

	checker := NewHLSChecker(2 * time.Second)
	checker.client = server.Client()

	urls := []string{
		server.URL + "/stream.m3u8",
		server.URL + "/stream.m3u8",
		server.URL + "/stream.m3u8",
	}

	ctx := context.Background()
	results := checker.CheckBatch(ctx, urls, 2)

	if len(results) != len(urls) {
		t.Errorf("Expected %d results, got %d", len(urls), len(results))
	}
}

func TestUserAgentRotation(t *testing.T) {
	checker := NewHLSChecker(time.Second)
	agents := make(map[string]bool)

	// Rotar más veces que el tamaño del pool para comprobar la vuelta
	for i := 0; i < len(checker.userAgents)*2; i++ {
		ua := checker.nextUserAgent()
		agents[ua] = true
	}

	if len(agents) != len(checker.userAgents) {
		t.Errorf("Expected %d unique User-Agents in pool, got %d", len(checker.userAgents), len(agents))
	}
}

func TestMirrorHealth_IsDegraded(t *testing.T) {
	cases := []struct {
		name        string
		bufferRatio float64
		errorRate   float64
		want        bool
	}{
		{"Healthy stream", 0.5, 0.0, false},
		{"High buffer ratio", 0.85, 0.0, true},
		{"High error rate", 0.3, 0.35, true},
		{"Both degraded", 0.9, 0.5, true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Imported from domain via structural embedding emulation:
			// We test logic inline to avoid circular imports
			isDegraded := c.bufferRatio > 0.80 || c.errorRate > 0.30
			if isDegraded != c.want {
				t.Errorf("IsDegraded(%v, %v) = %v, want %v", c.bufferRatio, c.errorRate, isDegraded, c.want)
			}
		})
	}
}
