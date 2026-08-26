package opensource

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gdberysan/open-tv/internal/domain"
)

// Provider implementa ports.ProviderPort para listas M3U públicas (IPTV-org)
// y, opcionalmente, para ficheros M3U locales subidos por el usuario (file://).
type Provider struct {
	id      string
	baseURL string
	client  *http.Client

	// maxBodyBytes limita el tamaño del M3U descargado/leído para evitar
	// consumo de memoria/disco descontrolado ante un proveedor hostil o roto.
	maxBodyBytes int64

	// allowedFileDir es el único directorio bajo el cual se permite abrir
	// un baseURL "file://". Vacío (por defecto) deshabilita file:// por
	// completo: es la guarda anti SSRF/path-traversal para esta fuente.
	allowedFileDir string

	// streamURLs almacena la URL de stream por channelID tras cada sync.
	// Las URLs de M3U público son estáticas, así que el caché en memoria
	// es suficiente para MVP; no se necesita persistencia en DB.
	mu         sync.RWMutex
	streamURLs map[domain.ChannelID]string
}

// Option configura parámetros opcionales de Provider en su construcción.
type Option func(*Provider)

// WithAllowedFileDir habilita la lectura de M3U locales vía baseURL
// "file://<ruta>", pero solo cuando <ruta> queda contenida dentro de dir.
// Sin esta opción (o con dir vacío) el soporte file:// permanece deshabilitado,
// que es el valor por defecto seguro para todo call site existente.
func WithAllowedFileDir(dir string) Option {
	return func(p *Provider) {
		p.allowedFileDir = dir
	}
}

const (
	// defaultClientTimeout cubre la descarga completa del M3U (~12k canales).
	defaultClientTimeout = 5 * time.Minute
	// defaultMaxM3UBytes: el índice completo de IPTV-org pesa unos pocos MB;
	// 50MB deja margen de sobra sin permitir descargas descontroladas.
	defaultMaxM3UBytes = 50 << 20
)

func NewProvider(id, baseURL string, client *http.Client, opts ...Option) *Provider {
	if client == nil {
		client = &http.Client{Timeout: defaultClientTimeout}
	}
	p := &Provider{
		id:           id,
		baseURL:      baseURL,
		client:       client,
		maxBodyBytes: defaultMaxM3UBytes,
		streamURLs:   make(map[domain.ChannelID]string),
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

func (p *Provider) ID() string                { return p.id }
func (p *Provider) Type() domain.ProviderType { return domain.ProviderOpenSource }

// GetLiveChannels obtiene el M3U (por HTTP o, si baseURL es "file://...", desde
// disco) y lo parsea en streaming línea a línea. Almacena la URL de cada
// stream en el caché interno para GetStreamURL.
func (p *Provider) GetLiveChannels(ctx context.Context) ([]domain.Channel, error) {
	if strings.HasPrefix(p.baseURL, "file://") {
		return p.getLiveChannelsFromFile()
	}
	return p.getLiveChannelsFromHTTP(ctx)
}

// getLiveChannelsFromHTTP es la vía original: descarga el M3U remoto.
func (p *Provider) getLiveChannelsFromHTTP(ctx context.Context) ([]domain.Channel, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL, nil)
	if err != nil {
		return nil, fmt.Errorf("opensource.GetLiveChannels (NewRequest): %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("opensource.GetLiveChannels (Do): %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("opensource.GetLiveChannels (Status): HTTP %d", resp.StatusCode)
	}

	channels, newURLs, err := parseM3UStream(resp.Body, p.id, p.maxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("opensource.GetLiveChannels: %w", err)
	}

	// Reemplazar caché completo al finalizar el parse
	p.mu.Lock()
	p.streamURLs = newURLs
	p.mu.Unlock()

	return channels, nil
}

// getLiveChannelsFromFile lee un M3U local (baseURL "file://<ruta>") y lo
// parsea con el mismo parser streaming que la vía HTTP, respetando el mismo
// cap de tamaño. La ruta debe quedar contenida en allowedFileDir.
func (p *Provider) getLiveChannelsFromFile() ([]domain.Channel, error) {
	path, err := p.resolveFilePath()
	if err != nil {
		return nil, fmt.Errorf("opensource.GetLiveChannels (file): %w", err)
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opensource.GetLiveChannels (file): no se pudo abrir %q: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	channels, newURLs, err := parseM3UStream(f, p.id, p.maxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("opensource.GetLiveChannels (file): %w", err)
	}

	// Reemplazar caché completo al finalizar el parse
	p.mu.Lock()
	p.streamURLs = newURLs
	p.mu.Unlock()

	return channels, nil
}

// resolveFilePath valida que la ruta indicada en baseURL ("file://<ruta>")
// quede contenida dentro de allowedFileDir y devuelve la ruta absoluta lista
// para os.Open. Es la guarda anti SSRF/path-traversal: allowedFileDir vacío
// deshabilita file:// por completo, y cualquier ruta que tras filepath.Clean
// quede fuera del prefijo permitido (incluyendo intentos con "../") es
// rechazada con un error envuelto, nunca con un pánico.
func (p *Provider) resolveFilePath() (string, error) {
	if p.allowedFileDir == "" {
		return "", fmt.Errorf("opensource: soporte file:// deshabilitado (no se configuró un directorio permitido)")
	}

	raw := strings.TrimPrefix(p.baseURL, "file://")
	if raw == "" {
		return "", fmt.Errorf("opensource: URL file:// vacía")
	}

	allowedAbs, err := filepath.Abs(filepath.Clean(p.allowedFileDir))
	if err != nil {
		return "", fmt.Errorf("opensource: no se pudo resolver el directorio permitido %q: %w", p.allowedFileDir, err)
	}
	pathAbs, err := filepath.Abs(filepath.Clean(raw))
	if err != nil {
		return "", fmt.Errorf("opensource: no se pudo resolver la ruta %q: %w", raw, err)
	}

	rel, err := filepath.Rel(allowedAbs, pathAbs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("opensource: ruta %q fuera del directorio permitido %q", pathAbs, allowedAbs)
	}

	return pathAbs, nil
}

// parseM3UStream parsea un M3U línea a línea desde r, con un cap de tamaño
// de maxBytes (io.LimitReader con N+1 para distinguir "justo en el límite" de
// "excedido"). Es el parser compartido entre la vía HTTP y la vía file://.
func parseM3UStream(r io.Reader, providerID string, maxBytes int64) ([]domain.Channel, map[domain.ChannelID]string, error) {
	limited := &io.LimitedReader{R: r, N: maxBytes + 1}
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
				ProviderID:   providerID,
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
			id := domain.ChannelID(providerID + "-" + currentChannel.Name)
			currentChannel.ID = id
			channels = append(channels, *currentChannel)
			newURLs[id] = line // line es la URL del stream
			currentChannel = nil
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, nil, fmt.Errorf("fallo al escanear M3U: %w", err)
	}
	if limited.N <= 0 {
		return nil, nil, fmt.Errorf("M3U excede el tamaño máximo de %d bytes", maxBytes)
	}

	return channels, newURLs, nil
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
	defer func() { _ = resp.Body.Close() }()
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
