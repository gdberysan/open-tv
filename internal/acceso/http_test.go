package acceso_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gdberysan/open-tv/internal/acceso"
)

func montar(t *testing.T) (http.Handler, *acceso.Sesiones) {
	t.Helper()
	s := acceso.NuevasSesiones("la-clave", nil)
	l := acceso.NuevoLimitador(5, time.Minute, nil)
	mux := http.NewServeMux()
	mux.Handle(acceso.RutaAcceso, acceso.Pagina(s, l))
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })
	mux.HandleFunc("/channels", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("[]")) })
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("<html>app</html>")) })
	return acceso.Exigir(s)(mux), s
}

func TestSinSesionLaAPIDa401YLaNavegacionVaAAcceso(t *testing.T) {
	h, _ := montar(t)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/channels", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("/channels sin sesión = %d, quiero 401", rec.Code)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != acceso.RutaAcceso {
		t.Errorf("/ sin sesión = %d → %q, quiero 303 → /acceso", rec.Code, rec.Header().Get("Location"))
	}
}

func TestHealthYAccesoNoExigenSesion(t *testing.T) {
	h, _ := montar(t)
	for _, ruta := range []string{"/health", acceso.RutaAcceso} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, ruta, nil))
		if rec.Code != http.StatusOK {
			t.Errorf("%s = %d, quiero 200", ruta, rec.Code)
		}
	}
}

// TestRutaCodificadaNoSeSaltaLaSesion cubre el bypass de ruta codificada: chi
// enruta por r.URL.RawPath cuando está fijado, así que "/acces%6F" no coincide
// con la ruta registrada /acceso y (en el router real) cae al fallback SPA. Si
// exenta() solo mirase Path (que sale decodificado a "/acceso"/"/health"), la
// daría por exenta y el mux de este test (que no tiene fallback SPA) la
// resolvería como 404 — de un modo u otro, sin pasar por conSesion(). Aquí se
// comprueba directamente el código 401 contra Exigir/montar().
func TestRutaCodificadaNoSeSaltaLaSesion(t *testing.T) {
	h, _ := montar(t)
	for _, ruta := range []string{"/acces%6F", "/healt%68"} {
		req := httptest.NewRequest(http.MethodGet, ruta, nil)
		if req.URL.RawPath == "" {
			t.Fatalf("%s: RawPath vacío, el test no prueba nada", ruta)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s sin sesión = %d, quiero 401", ruta, rec.Code)
		}
	}
}

func postClave(h http.Handler, clave, ip string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, acceso.RutaAcceso, strings.NewReader(url.Values{"clave": {clave}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.RemoteAddr = ip + ":5555"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestClaveBuenaCreaSesionYAbreLaAPI(t *testing.T) {
	h, _ := montar(t)

	rec := postClave(h, "la-clave", "10.0.0.2")
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/" {
		t.Fatalf("POST clave buena = %d → %q, quiero 303 → /", rec.Code, rec.Header().Get("Location"))
	}
	var cookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == acceso.NombreCookie {
			cookie = c
		}
	}
	if cookie == nil {
		t.Fatal("no hay cookie de sesión")
	}
	if !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.Path != "/" {
		t.Errorf("cookie %+v: quiero HttpOnly, SameSite=Strict y Path=/", cookie)
	}
	if cookie.Secure {
		t.Error("Secure en una petición http sin X-Forwarded-Proto: el navegador no la devolvería")
	}

	req := httptest.NewRequest(http.MethodGet, "/channels", nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("/channels con sesión = %d, quiero 200", rec.Code)
	}
}

func TestClaveBuenaDetrasDeHTTPSMarcaLaCookieSecure(t *testing.T) {
	h, _ := montar(t)
	req := httptest.NewRequest(http.MethodPost, acceso.RutaAcceso, strings.NewReader("clave=la-clave"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-Forwarded-Proto", "https")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var cookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == acceso.NombreCookie {
			cookie = c
		}
	}
	if cookie == nil {
		t.Fatal("no hay cookie de sesión: el test no prueba nada sin ella")
	}
	if !cookie.Secure {
		t.Error("detrás de HTTPS la cookie no lleva Secure")
	}
}

func TestClaveMalaNoCreaSesion(t *testing.T) {
	h, _ := montar(t)
	rec := postClave(h, "otra", "10.0.0.2")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("POST clave mala = %d, quiero 401", rec.Code)
	}
	if len(rec.Result().Cookies()) != 0 {
		t.Error("una clave mala dejó cookie")
	}
	if !strings.Contains(rec.Body.String(), `role="alert"`) {
		t.Error("el error no se anuncia (falta role=alert)")
	}
}

func TestDemasiadosIntentosDan429(t *testing.T) {
	h, _ := montar(t)
	for i := 0; i < 5; i++ {
		postClave(h, "otra", "10.0.0.9")
	}
	if rec := postClave(h, "la-clave", "10.0.0.9"); rec.Code != http.StatusTooManyRequests {
		t.Errorf("sexto intento = %d, quiero 429 aunque la clave sea buena", rec.Code)
	}
}

func TestPaginaDeAccesoEnInglesYConCSPSinScripts(t *testing.T) {
	h, _ := montar(t)
	req := httptest.NewRequest(http.MethodGet, acceso.RutaAcceso, nil)
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	cuerpo := rec.Body.String()
	if !strings.Contains(cuerpo, `lang="en"`) || !strings.Contains(cuerpo, "Access key") {
		t.Errorf("la página no sale en inglés:\n%s", cuerpo)
	}
	if strings.Contains(cuerpo, "<script") {
		t.Error("la página de acceso lleva scripts")
	}
	csp := rec.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "default-src 'none'") || !strings.Contains(csp, "form-action 'self'") {
		t.Errorf("CSP %q", csp)
	}
	if !strings.Contains(cuerpo, `<label for="clave"`) || !strings.Contains(cuerpo, `autocomplete="current-password"`) {
		t.Error("el campo no tiene etiqueta asociada o autocompletado de contraseña")
	}
}

func TestPaginaDeAccesoPorDefectoEnEspanol(t *testing.T) {
	h, _ := montar(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, acceso.RutaAcceso, nil))
	if !strings.Contains(rec.Body.String(), `lang="es"`) || !strings.Contains(rec.Body.String(), "Clave de acceso") {
		t.Errorf("sin Accept-Language no sale en español:\n%s", rec.Body.String())
	}
}

func TestClaveConEspaciosAlrededorEntra(t *testing.T) {
	h, _ := montar(t)
	rec := postClave(h, "la-clave ", "10.0.0.3")
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/" {
		t.Fatalf("POST clave con espacio final = %d → %q, quiero 303 → /", rec.Code, rec.Header().Get("Location"))
	}
	var cookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == acceso.NombreCookie {
			cookie = c
		}
	}
	if cookie == nil {
		t.Fatal("no hay cookie de sesión")
	}
}
