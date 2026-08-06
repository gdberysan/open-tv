package healthcheck

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// HLSSegmentResult contiene las métricas derivadas de una verificación de salud HLS.
type HLSSegmentResult struct {
	MirrorURL   string
	IsAlive     bool
	LatencyMs   int64
	BitrateKbps int64
	BufferRatio float64 // download_time / segment_duration (>0.8 = en riesgo)
	JitterMs    int64
	Error       error
}

// HLSChecker realiza health-checks predictivos sobre streams HLS.
type HLSChecker struct {
	client     *http.Client
	userAgents []string // Pool rotatorio para evitar bloqueos por user-agent
	uaIndex    int
	uaMu       sync.Mutex
}

// NewHLSChecker crea un checker con un pool de User-Agents pre-configurado.
func NewHLSChecker(timeout time.Duration) *HLSChecker {
	return &HLSChecker{
		client: &http.Client{Timeout: timeout},
		// Pool de User-Agents comunes de reproductores multimedia
		// para evitar que el servidor de origen bloquee el health-checker.
		userAgents: []string{
			"Mozilla/5.0 (compatible; IPTV-Bot/1.0)",
			"VLC/3.0.20 LibVLC/3.0.20",
			"ExoPlayer/2.18.0 (Linux; Android 13)",
			"Lavf/58.76.100",
			"HLS.js/1.4.0",
		},
	}
}

// nextUserAgent devuelve el siguiente User-Agent del pool de forma thread-safe (round-robin).
func (h *HLSChecker) nextUserAgent() string {
	h.uaMu.Lock()
	defer h.uaMu.Unlock()
	ua := h.userAgents[h.uaIndex%len(h.userAgents)]
	h.uaIndex++
	return ua
}

// CheckOne realiza la verificación de salud de un único mirror HLS.
// Estrategia: obtiene el .m3u8, extrae el primer segmento .ts y mide
// cuánto tarda en descargar ese segmento vs la duración declarada.
func (h *HLSChecker) CheckOne(ctx context.Context, mirrorURL string) HLSSegmentResult {
	result := HLSSegmentResult{MirrorURL: mirrorURL}
	start := time.Now()

	// 1. Descargar el playlist HLS
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, mirrorURL, nil)
	if err != nil {
		result.Error = fmt.Errorf("healthcheck.CheckOne (NewRequest): %w", err)
		return result
	}
	req.Header.Set("User-Agent", h.nextUserAgent())

	resp, err := h.client.Do(req)
	if err != nil {
		result.Error = fmt.Errorf("healthcheck.CheckOne (playlist fetch): %w", err)
		return result
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		result.Error = fmt.Errorf("healthcheck.CheckOne (status %d)", resp.StatusCode)
		return result
	}

	// 2. Parsear el .m3u8 para encontrar el primer segmento y su duración
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		result.Error = fmt.Errorf("healthcheck.CheckOne (read playlist): %w", err)
		return result
	}

	segmentURL, segmentDuration, err := extractFirstSegment(mirrorURL, string(body))
	if err != nil {
		// No es un error fatal; podría ser un master playlist que redirige
		result.IsAlive = true
		result.LatencyMs = time.Since(start).Milliseconds()
		return result
	}

	// 3. Descargar el primer segmento .ts y medir tiempo vs duración declarada
	segStart := time.Now()
	segReq, err := http.NewRequestWithContext(ctx, http.MethodGet, segmentURL, nil)
	if err != nil {
		result.Error = fmt.Errorf("healthcheck.CheckOne (segment request): %w", err)
		return result
	}
	segReq.Header.Set("User-Agent", h.nextUserAgent())

	segResp, err := h.client.Do(segReq)
	if err != nil {
		result.Error = fmt.Errorf("healthcheck.CheckOne (segment fetch): %w", err)
		return result
	}
	defer segResp.Body.Close()

	// Leer sin cargar en RAM: contamos bytes con io.Discard
	n, err := io.Copy(io.Discard, segResp.Body)
	if err != nil {
		result.Error = fmt.Errorf("healthcheck.CheckOne (segment read): %w", err)
		return result
	}
	segDownloadTime := time.Since(segStart)

	// 4. Calcular métricas
	result.IsAlive = true
	result.LatencyMs = time.Since(start).Milliseconds()
	result.BitrateKbps = (n * 8) / max(segDownloadTime.Milliseconds(), 1)

	if segmentDuration > 0 {
		// Buffer ratio: cuánto del presupuesto de tiempo consumimos descargando
		result.BufferRatio = float64(segDownloadTime.Milliseconds()) / float64(segmentDuration*1000)
	}

	return result
}

// CheckBatch valida un conjunto de mirrors en paralelo con concurrencia acotada.
// maxWorkers evita ahogar la red; un valor de 20 es razonable para health-checks.
func (h *HLSChecker) CheckBatch(ctx context.Context, mirrorURLs []string, maxWorkers int) []HLSSegmentResult {
	results := make([]HLSSegmentResult, len(mirrorURLs))

	// Semáforo para acotar la concurrencia (Riesgo #4 mitigado)
	sem := make(chan struct{}, maxWorkers)
	var wg sync.WaitGroup

	for i, url := range mirrorURLs {
		wg.Add(1)
		go func(idx int, u string) {
			defer wg.Done()

			// Adquirir token del semáforo
			sem <- struct{}{}
			defer func() { <-sem }()

			results[idx] = h.CheckOne(ctx, u)
		}(i, url)
	}

	wg.Wait()
	return results
}

// extractFirstSegment parsea un .m3u8 y retorna la URL y duración del primer segmento .ts.
func extractFirstSegment(baseURL, playlist string) (string, float64, error) {
	lines := strings.Split(playlist, "\n")
	var duration float64

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#EXTINF:") {
			// Extraer duración: "#EXTINF:6.006,"
			durStr := strings.TrimPrefix(line, "#EXTINF:")
			durStr = strings.Split(durStr, ",")[0]
			d, err := strconv.ParseFloat(durStr, 64)
			if err == nil {
				duration = d
			}
			continue
		}
		// La siguiente línea no-comentario es la URL del segmento
		if !strings.HasPrefix(line, "#") && line != "" {
			segURL := line
			if !strings.HasPrefix(segURL, "http") {
				// URL relativa → construir absoluta desde el base
				base := baseURL[:strings.LastIndex(baseURL, "/")+1]
				segURL = base + segURL
			}
			return segURL, duration, nil
		}
	}

	return "", 0, fmt.Errorf("no se encontraron segmentos en el playlist")
}

// max devuelve el mayor de dos int64 (helper para evitar división por cero).
func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
