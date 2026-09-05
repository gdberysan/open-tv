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

	"github.com/gdberysan/open-tv/internal/adapters/providers/iptvorg"
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

	// tvgIDs guarda el tvg-id de cada canal del último GetLiveChannels: es la
	// clave con la que se consulta la API. Protegido por mu.
	tvgIDs map[domain.ChannelID]string

	// tvgURLs son las URLs de guía EPG (url-tvg / x-tvg-url) que la fuente
	// declaró en la cabecera #EXTM3U de la última llamada a GetLiveChannels.
	// Vacío si la fuente no declaró ninguna. Protegido por mu.
	tvgURLs []string

	// enriquecedor es nil salvo que la fuente sea iptv-org.
	enriquecedor Enriquecedor
}

// Enriquecedor aporta lo que el M3U no trae. Es OPCIONAL: sin él (el caso de un
// M3U subido por el usuario, que no tiene API detrás) el provider se comporta
// exactamente igual que siempre.
type Enriquecedor interface {
	Streams(canal, feed string) []iptvorg.StreamExtra
	Categoria(canal string) (string, bool)
}

// WithEnriquecedor conecta la API de iptv-org. Solo debe usarse cuando la
// fuente ES iptv-org: para cualquier otro M3U los identificadores no casan.
func WithEnriquecedor(e Enriquecedor) Option {
	return func(p *Provider) {
		p.enriquecedor = e
	}
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
		tvgIDs:       make(map[domain.ChannelID]string),
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

	result, err := parseM3UStream(resp.Body, p.id, p.maxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("opensource.GetLiveChannels: %w", err)
	}

	// Reemplazar caché completo al finalizar el parse
	p.mu.Lock()
	p.streamURLs = result.streamURLs
	p.tvgIDs = result.tvgIDs
	p.tvgURLs = result.tvgURLs
	p.mu.Unlock()

	return result.channels, nil
}

// getLiveChannelsFromFile lee un M3U local (baseURL "file://<ruta>") y lo
// parsea con el mismo parser streaming que la vía HTTP, respetando el mismo
// cap de tamaño. La ruta debe quedar contenida en allowedFileDir.
func (p *Provider) getLiveChannelsFromFile() ([]domain.Channel, error) {
	path, err := p.resolveFilePath()
	if err != nil {
		return nil, fmt.Errorf("opensource.GetLiveChannels (file): %w", err)
	}

	f, err := os.Open(path) //nolint:gosec // path viene de resolveFilePath(), con guarda de contención (Abs+Clean+Rel bajo el dir permitido); no es entrada de usuario sin validar
	if err != nil {
		return nil, fmt.Errorf("opensource.GetLiveChannels (file): no se pudo abrir %q: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	result, err := parseM3UStream(f, p.id, p.maxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("opensource.GetLiveChannels (file): %w", err)
	}

	// Reemplazar caché completo al finalizar el parse
	p.mu.Lock()
	p.streamURLs = result.streamURLs
	p.tvgIDs = result.tvgIDs
	p.tvgURLs = result.tvgURLs
	p.mu.Unlock()

	return result.channels, nil
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

// m3uParseResult agrupa las tres salidas de parseM3UStream. Se usa un struct
// (en vez de un tuple de 4 valores) porque es más legible en los call sites y
// deja margen para que crezca sin volver a tocar todas las firmas.
type m3uParseResult struct {
	channels   []domain.Channel
	streamURLs map[domain.ChannelID]string
	tvgIDs     map[domain.ChannelID]string
	tvgURLs    []string
}

// parseM3UStream parsea un M3U línea a línea desde r, con un cap de tamaño
// de maxBytes (io.LimitReader con N+1 para distinguir "justo en el límite" de
// "excedido"). Es el parser compartido entre la vía HTTP y la vía file://.
func parseM3UStream(r io.Reader, providerID string, maxBytes int64) (m3uParseResult, error) {
	limited := &io.LimitedReader{R: r, N: maxBytes + 1}
	scanner := bufio.NewScanner(limited)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	var (
		channels       []domain.Channel
		currentChannel *domain.Channel
		newURLs        = make(map[domain.ChannelID]string)
		newTvgIDs      = make(map[domain.ChannelID]string)
		tvgURLs        []string
		firstLine      = true
	)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		if firstLine {
			firstLine = false
			if strings.HasPrefix(line, "#EXTM3U") {
				tvgURLs = extractTvgURLs(line)
				continue
			}
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
			newTvgIDs[id] = currentChannel.TvgID
			currentChannel = nil
		}
	}

	if err := scanner.Err(); err != nil {
		return m3uParseResult{}, fmt.Errorf("fallo al escanear M3U: %w", err)
	}
	if limited.N <= 0 {
		return m3uParseResult{}, fmt.Errorf("M3U excede el tamaño máximo de %d bytes", maxBytes)
	}

	return m3uParseResult{channels: channels, streamURLs: newURLs, tvgIDs: newTvgIDs, tvgURLs: tvgURLs}, nil
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

// GetStreamsDeCanal devuelve la URL del M3U PRIMERO y, si hay enriquecedor, los
// mirrors de la API detrás, sin duplicados. El orden de inserción solo decide el
// arranque en frío: a partir de ahí manda la salud (FindMirrorsByChannelID).
func (p *Provider) GetStreamsDeCanal(_ context.Context, channelID domain.ChannelID) ([]iptvorg.StreamExtra, error) {
	p.mu.RLock()
	url, ok := p.streamURLs[channelID]
	tvgID := p.tvgIDs[channelID]
	enr := p.enriquecedor
	p.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("opensource.GetStreamsDeCanal: canal %s no encontrado (sync pendiente?)", channelID)
	}

	salida := []iptvorg.StreamExtra{{URL: url}}
	if enr == nil || tvgID == "" {
		return salida, nil
	}

	// vistas apunta a la POSICIÓN en salida, no a un booleano: cuando la API
	// repite una URL que ya tenemos, hay que poder volver a esa fila para
	// ADOPTAR sus cabeceras. La URL del M3U entra sin ellas (el M3U no las
	// lleva) y el M3U y la API son del mismo proyecto, así que la principal
	// casi siempre está repetida: descartar la entrada de la API sin más
	// tiraba las cabeceras justo en el stream que se intenta PRIMERO —
	// medido sobre datos reales, 753 de 982 streams con cabeceras las perdían.
	vistas := map[string]int{url: 0}
	canal, feed := iptvorg.SepararTvgID(tvgID)
	for _, s := range enr.Streams(canal, feed) {
		if s.URL == "" {
			continue
		}
		if i, dup := vistas[s.URL]; dup {
			// No se añade fila nueva, pero sí se rellena lo que falta. Las dos
			// cabeceras viajan JUNTAS (son la protección de hotlink de un
			// mismo origen) y solo si la fila que ya está no trae ninguna: una
			// entrada que ya declaró las suyas nunca se pisa.
			if salida[i].Referrer == "" && salida[i].UserAgent == "" {
				salida[i].Referrer = s.Referrer
				salida[i].UserAgent = s.UserAgent
			}
			continue
		}
		vistas[s.URL] = len(salida)
		salida = append(salida, s)
	}
	return salida, nil
}

// CategoriaDe devuelve la categoría upstream del canal cuyo tvg_id se pasa.
// Sin enriquecedor, o sin tvg_id, no hay categoría.
func (p *Provider) CategoriaDe(tvgID string) (string, bool) {
	p.mu.RLock()
	enr := p.enriquecedor
	p.mu.RUnlock()
	if enr == nil || tvgID == "" {
		return "", false
	}
	canal, _ := iptvorg.SepararTvgID(tvgID)
	return enr.Categoria(canal)
}

// TvgURLs devuelve las URLs de guía EPG (url-tvg / x-tvg-url) que la fuente
// declaró en la cabecera #EXTM3U durante la última llamada a GetLiveChannels.
// Se puebla tras GetLiveChannels; antes de la primera llamada (o si la fuente
// no declaró guía) devuelve un slice vacío, nunca nil.
func (p *Provider) TvgURLs() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]string, len(p.tvgURLs))
	copy(out, p.tvgURLs)
	return out
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

// extractTvgURLs extrae las URLs de guía EPG que la cabecera #EXTM3U declara
// en su atributo url-tvg (estándar) o, como alias, x-tvg-url. El valor puede
// ser una lista separada por comas; cada elemento se recorta con TrimSpace y
// los vacíos se descartan. Devuelve nil si no hay atributo de guía.
func extractTvgURLs(headerLine string) []string {
	raw := extractAttr(headerLine, "url-tvg")
	if raw == "" {
		raw = extractAttr(headerLine, "x-tvg-url")
	}
	if raw == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	urls := make([]string, 0, len(parts))
	for _, part := range parts {
		if u := strings.TrimSpace(part); u != "" {
			urls = append(urls, u)
		}
	}
	return urls
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
