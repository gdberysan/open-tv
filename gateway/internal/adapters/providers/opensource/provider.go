package opensource

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/tu-org/iptv-ecosystem/gateway/internal/domain"
)

// Provider implementa ports.ProviderPort para listas M3U públicas (IPTV-org).
type Provider struct {
	id      string
	baseURL string
	client  *http.Client

	// maxBodyBytes limita el tamaño del M3U descargado para evitar
	// consumo de memoria/disco descontrolado ante un proveedor hostil o roto.
	maxBodyBytes int64

	// streamURLs almacena la URL de stream por channelID tras cada sync.
	// Las URLs de M3U público son estáticas, así que el caché en memoria
	// es suficiente para MVP; no se necesita persistencia en DB.
	mu         sync.RWMutex
	streamURLs map[domain.ChannelID]string
}

const (
	// defaultClientTimeout cubre la descarga completa del M3U (~12k canales).
	defaultClientTimeout = 5 * time.Minute
	// defaultMaxM3UBytes: el índice completo de IPTV-org pesa unos pocos MB;
	// 50MB deja margen de sobra sin permitir descargas descontroladas.
	defaultMaxM3UBytes = 50 << 20
)

func NewProvider(id, baseURL string, client *http.Client) *Provider {
	if client == nil {
		client = &http.Client{Timeout: defaultClientTimeout}
	}
	return &Provider{
		id:           id,
		baseURL:      baseURL,
		client:       client,
		maxBodyBytes: defaultMaxM3UBytes,
		streamURLs:   make(map[domain.ChannelID]string),
	}
}

func (p *Provider) ID() string                { return p.id }
func (p *Provider) Type() domain.ProviderType { return domain.ProviderOpenSource }

// GetLiveChannels descarga y parsea el M3U en streaming línea a línea.
// Almacena la URL de cada stream en el caché interno para GetStreamURL.
func (p *Provider) GetLiveChannels(ctx context.Context) ([]domain.Channel, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL, nil)
	if err != nil {
		return nil, fmt.Errorf("opensource.GetLiveChannels (NewRequest): %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("opensource.GetLiveChannels (Do): %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("opensource.GetLiveChannels (Status): HTTP %d", resp.StatusCode)
	}

	// N+1 para distinguir "justo en el límite" de "excedido"
	limited := &io.LimitedReader{R: resp.Body, N: p.maxBodyBytes + 1}
	scanner := bufio.NewScanner(limited)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	var (
		channels       []domain.Channel
		currentChannel *domain.Channel
		newURLs        = make(map[domain.ChannelID]string)
	)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "#EXTINF:") {
			currentChannel = &domain.Channel{
				ProviderID:   p.id,
				ProviderType: domain.ProviderOpenSource,
			}
			currentChannel.Name = extractAfterComma(line)
			currentChannel.TvgID = extractAttr(line, "tvg-id")
			currentChannel.LogoURL = extractAttr(line, "tvg-logo")
			currentChannel.CategoryID = extractAttr(line, "group-title")
			currentChannel.LanguageCode = strings.ToLower(extractAttr(line, "tvg-language"))
			// tvg-country explícito tiene prioridad; si no, derivar del tvg-id (ej: "BBC.uk@SD" → "GB")
			if cc := extractAttr(line, "tvg-country"); cc != "" {
				currentChannel.CountryCode = strings.ToUpper(strings.SplitN(cc, "|", 2)[0])
			} else {
				currentChannel.CountryCode = countryFromTvgID(extractAttr(line, "tvg-id"))
			}

		} else if !strings.HasPrefix(line, "#") && currentChannel != nil {
			id := domain.ChannelID(p.id + "-" + currentChannel.Name)
			currentChannel.ID = id
			channels = append(channels, *currentChannel)
			newURLs[id] = line // line es la URL del stream
			currentChannel = nil
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("opensource.GetLiveChannels (Scan): %w", err)
	}
	if limited.N <= 0 {
		return nil, fmt.Errorf("opensource.GetLiveChannels: M3U excede el tamaño máximo de %d bytes", p.maxBodyBytes)
	}

	// Reemplazar caché completo al finalizar el parse
	p.mu.Lock()
	p.streamURLs = newURLs
	p.mu.Unlock()

	return channels, nil
}

// GetStreamURL retorna la URL del stream desde el caché poblado por GetLiveChannels.
func (p *Provider) GetStreamURL(_ context.Context, channelID domain.ChannelID) (string, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	u, ok := p.streamURLs[channelID]
	if !ok {
		return "", fmt.Errorf("opensource.GetStreamURL: canal %s no encontrado (sync pendiente?)", channelID)
	}
	return u, nil
}

func (p *Provider) HealthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, p.baseURL, nil)
	if err != nil {
		return fmt.Errorf("opensource.HealthCheck (NewRequest): %w", err)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("opensource.HealthCheck (Do): %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 && resp.StatusCode != http.StatusMethodNotAllowed {
		return fmt.Errorf("opensource.HealthCheck (Status): %d", resp.StatusCode)
	}
	return nil
}

// ── helpers de parseo M3U ────────────────────────────────────────────────────

// countryFromTvgID extrae el país del tvg-id de IPTV-org.
// Formato típico: "BBCNews.uk@HD" o "France2.fr@SD"
// El TLD se convierte a ISO 3166-1 alpha-2 con correcciones para casos especiales.
func countryFromTvgID(tvgID string) string {
	if tvgID == "" {
		return ""
	}
	// Extraer parte antes de "@"
	base := strings.SplitN(tvgID, "@", 2)[0]
	// Extraer TLD (parte tras el último punto)
	dot := strings.LastIndex(base, ".")
	if dot < 0 || dot == len(base)-1 {
		return ""
	}
	tld := strings.ToUpper(base[dot+1:])
	// Correcciones de TLD → ISO 3166-1 alpha-2
	overrides := map[string]string{
		"UK": "GB",
	}
	if iso, ok := overrides[tld]; ok {
		return iso
	}
	if len(tld) == 2 {
		return tld
	}
	return ""
}

func extractAfterComma(line string) string {
	if i := strings.LastIndex(line, ","); i != -1 {
		return strings.TrimSpace(line[i+1:])
	}
	return ""
}

func extractAttr(line, attr string) string {
	key := attr + `="`
	idx := strings.Index(line, key)
	if idx == -1 {
		return ""
	}
	sub := line[idx+len(key):]
	end := strings.Index(sub, `"`)
	if end == -1 {
		return ""
	}
	return sub[:end]
}
