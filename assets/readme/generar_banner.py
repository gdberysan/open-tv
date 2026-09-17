"""Genera las imágenes del README: assets/readme/banner-{es,en}.svg (cabecera)
y diagrama-{es,en}.svg (cómo funciona).

El texto se convierte a trazos con las fuentes de la marca (web/public/fonts),
así que el SVG no depende de fuentes instaladas y se ve igual en GitHub. La
paleta es la de Korven y la animación CSS se apaga con prefers-reduced-motion.

Herramienta de mantenimiento, NO dependencia del build. Para regenerar:

    python3 -m venv /tmp/banner && /tmp/banner/bin/pip install fonttools brotli
    /tmp/banner/bin/python assets/readme/generar_banner.py
"""
import os
from fontTools.ttLib import TTFont
from fontTools.varLib.instancer import instantiateVariableFont
from fontTools.pens.svgPathPen import SVGPathPen
from fontTools.pens.transformPen import TransformPen

RAIZ = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
FONTS = os.path.join(RAIZ, "web", "public", "fonts") + os.sep
_cache = {}


def font(name, wght):
    key = (name, wght)
    if key not in _cache:
        f = TTFont(FONTS + name)
        _cache[key] = instantiateVariableFont(f, {"wght": wght})
    return _cache[key]


def text(s, x, y, size, name, wght, tracking=0.0):
    """Devuelve (d, ancho) del texto con la línea base en y."""
    f = font(name, wght)
    gs = f.getGlyphSet()
    cmap = f.getBestCmap()
    upm = f["head"].unitsPerEm
    scale = size / upm
    pen = SVGPathPen(gs, ntos=lambda v: ("%.1f" % v).rstrip("0").rstrip("."))
    cx = 0.0
    for ch in s:
        g = cmap.get(ord(ch))
        if g is None:
            continue
        tp = TransformPen(pen, (scale, 0, 0, -scale, x + cx, y))
        gs[g].draw(tp)
        cx += gs[g].width * scale + tracking * size
    return pen.getCommands(), cx - tracking * size


def width(s, size, name, wght, tracking=0.0):
    return text(s, 0, 0, size, name, wght, tracking)[1]


SG = "space-grotesk-latin-wght-normal.woff2"
MONO = "jetbrains-mono-latin-wght-normal.woff2"
INTER = "inter-latin-wght-normal.woff2"

COPY = {
    "es": {
        "kicker": "TELEVISIÓN ABIERTA",
        "tag": "Tus listas M3U. Un solo binario. Sin cuentas ni nube.",
        "live": "EN VIVO",
        "chips": ["sin telemetría", "AirPlay", "guía EPG", "macOS · Linux · Windows"],
        "label": "Korven Open TV — televisión abierta, sin cuentas ni nube, en tu máquina",
    },
    "en": {
        "kicker": "FREE-TO-AIR TELEVISION",
        "tag": "Your M3U lists. One binary. No accounts, no cloud.",
        "live": "LIVE",
        "chips": ["no telemetry", "AirPlay", "EPG guide", "macOS · Linux · Windows"],
        "label": "Korven Open TV — free-to-air television, no accounts, no cloud, on your machine",
    },
}

GRAFITO, CARBON, AMBAR, ACERO, HUESO, LINEA, OK = (
    "#0E131B", "#171E29", "#FF8A2B", "#97A3B2", "#EFF3F8", "#283142", "#4FB286")


def build(lang):
    c = COPY[lang]
    W, H = 1280, 440
    out = []
    a = out.append
    a(f'<svg xmlns="http://www.w3.org/2000/svg" width="{W}" height="{H}" viewBox="0 0 {W} {H}" role="img" aria-label="{c["label"]}">')
    a(f"<title>{c['label']}</title>")
    a("""<style>
.pulse{animation:pulse 2.4s ease-out infinite;transform-origin:center;transform-box:fill-box}
.ring{animation:ring 2.4s ease-out infinite;transform-origin:center;transform-box:fill-box}
.blink{animation:blink 1.6s steps(2,start) infinite}
.sweep{animation:sweep 5s linear infinite}
.bar{animation:eq 1.4s ease-in-out infinite;transform-box:fill-box;transform-origin:bottom}
.wave{stroke-dasharray:14 10;animation:dash 1.8s linear infinite}
@keyframes pulse{0%,100%{transform:scale(1)}50%{transform:scale(1.12)}}
@keyframes ring{0%{transform:scale(.6);opacity:.9}100%{transform:scale(2.4);opacity:0}}
@keyframes blink{to{visibility:hidden}}
@keyframes sweep{0%{transform:translateY(-40px)}100%{transform:translateY(300px)}}
@keyframes eq{0%,100%{transform:scaleY(.35)}50%{transform:scaleY(1)}}
@keyframes dash{to{stroke-dashoffset:-48}}
@media (prefers-reduced-motion:reduce){*{animation:none!important}}
</style>""")
    a(f"""<defs>
<radialGradient id="glow" cx="0.28" cy="0.5" r="0.55"><stop offset="0" stop-color="{AMBAR}" stop-opacity=".16"/><stop offset="1" stop-color="{AMBAR}" stop-opacity="0"/></radialGradient>
<radialGradient id="glow2" cx="0.85" cy="0.2" r="0.5"><stop offset="0" stop-color="#5E9BD6" stop-opacity=".10"/><stop offset="1" stop-color="#5E9BD6" stop-opacity="0"/></radialGradient>
<pattern id="dots" width="22" height="22" patternUnits="userSpaceOnUse"><circle cx="1" cy="1" r="1" fill="{LINEA}"/></pattern>
<linearGradient id="fade" x1="0" x2="1"><stop offset="0" stop-color="#fff" stop-opacity="0"/><stop offset=".35" stop-color="#fff" stop-opacity=".55"/><stop offset="1" stop-color="#fff" stop-opacity=".15"/></linearGradient>
<mask id="dotmask"><rect width="{W}" height="{H}" fill="url(#fade)"/></mask>
<linearGradient id="screen" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="#1B2432"/><stop offset="1" stop-color="#0B1017"/></linearGradient>
<linearGradient id="scan" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="{AMBAR}" stop-opacity="0"/><stop offset=".5" stop-color="{AMBAR}" stop-opacity=".22"/><stop offset="1" stop-color="{AMBAR}" stop-opacity="0"/></linearGradient>
<clipPath id="card"><rect width="{W}" height="{H}" rx="24"/></clipPath>
<clipPath id="tv"><rect x="820" y="70" width="380" height="238" rx="14"/></clipPath>
</defs>""")
    a('<g clip-path="url(#card)">')
    a(f'<rect width="{W}" height="{H}" fill="{GRAFITO}"/>')
    a(f'<rect width="{W}" height="{H}" fill="url(#dots)" mask="url(#dotmask)"/>')
    a(f'<rect width="{W}" height="{H}" fill="url(#glow)"/><rect width="{W}" height="{H}" fill="url(#glow2)"/>')

    # --- emblema ---
    ex, ey, es = 72, 78, 0.62  # 128px -> ~79px
    a(f'<g transform="translate({ex} {ey}) scale({es})">')
    a(f'<polygon points="64,12 110,38 110,90 64,116 18,90 18,38" fill="{CARBON}" stroke="{HUESO}" stroke-width="3" stroke-linejoin="round"/>')
    a(f'<g stroke="{LINEA}" stroke-width="2" stroke-linejoin="round" fill="none"><polyline points="64,12 64,64 110,38"/><polyline points="64,64 110,90"/><polyline points="64,64 18,90"/><polyline points="64,64 18,38"/><polyline points="64,64 64,116"/></g>')
    a(f'<polyline points="40,52 28,64 40,76" fill="none" stroke="{ACERO}" stroke-width="3" stroke-linejoin="round" stroke-linecap="round"/>')
    a(f'<circle class="ring" cx="64" cy="64" r="9" fill="none" stroke="{AMBAR}" stroke-width="3"/>')
    a(f'<circle class="pulse" cx="64" cy="64" r="9" fill="{AMBAR}"/>')
    a("</g>")

    # kicker junto al emblema
    d, _ = text(c["kicker"], 170, 126, 16, MONO, 500, tracking=0.2)
    a(f'<path d="{d}" fill="{ACERO}"/>')

    # --- wordmark ---
    d1, w1 = text("KORVEN", 70, 244, 80, SG, 700, tracking=-0.02)
    a(f'<path d="{d1}" fill="{HUESO}"/>')
    d2, w2 = text("OPEN TV", 70 + w1 + 24, 244, 80, SG, 700, tracking=-0.02)
    a(f'<path d="{d2}" fill="{AMBAR}"/>')

    # tagline
    d, _ = text(c["tag"], 73, 298, 25, INTER, 450)
    a(f'<path d="{d}" fill="{ACERO}"/>')

    # chips
    x = 74
    for label in c["chips"]:
        tw = width(label, 15, MONO, 500)
        a(f'<rect x="{x}" y="340" width="{tw + 28:.1f}" height="34" rx="17" fill="{CARBON}" stroke="{LINEA}"/>')
        d, _ = text(label, x + 14, 362, 15, MONO, 500)
        a(f'<path d="{d}" fill="{HUESO}"/>')
        x += tw + 28 + 10

    # --- televisor ---
    a(f'<rect x="812" y="62" width="396" height="254" rx="20" fill="{CARBON}" stroke="{LINEA}" stroke-width="2"/>')
    a('<g clip-path="url(#tv)">')
    a('<rect x="820" y="70" width="380" height="238" fill="url(#screen)"/>')
    # onda de señal
    a(f'<path class="wave" d="M840 230 C 880 170, 920 290, 960 230 S 1040 170, 1080 230 S 1160 290, 1190 230" fill="none" stroke="{AMBAR}" stroke-opacity=".55" stroke-width="3" stroke-linecap="round"/>')
    # ecualizador
    bx = 846
    for i, h in enumerate([30, 52, 38, 64, 46, 28, 56, 40, 50, 34]):
        a(f'<rect class="bar" style="animation-delay:-{i * 0.17:.2f}s" x="{1030 + i * 16}" y="{258 - h}" width="9" height="{h}" rx="3" fill="{HUESO}" fill-opacity=".14"/>')
    d, _ = text("Canal 7" if lang == "es" else "Channel 7", 842, 288, 26, SG, 600)
    a(f'<path d="{d}" fill="{HUESO}"/>')
    a(f'<rect x="842" y="298" width="336" height="3" rx="1.5" fill="{LINEA}"/><rect x="842" y="298" width="336" height="3" rx="1.5" fill="{AMBAR}" fill-opacity=".8"/>')
    a('<rect class="sweep" x="820" y="70" width="380" height="40" fill="url(#scan)"/>')
    a("</g>")
    # overlay «en vivo»
    a(f'<circle class="blink" cx="848" cy="100" r="6" fill="#E5604D"/>')
    d, lw = text(c["live"], 862, 106, 17, INTER, 600)
    a(f'<path d="{d}" fill="{HUESO}"/>')
    d, _ = text("720p · 146 ms", 862 + lw + 12, 106, 15, MONO, 400)
    a(f'<path d="{d}" fill="{ACERO}"/>')
    # pie del televisor
    a(f'<rect x="990" y="316" width="36" height="22" fill="{CARBON}"/><rect x="940" y="338" width="136" height="8" rx="4" fill="{LINEA}"/>')

    # tira de canales
    chans = [("232 ms", OK), ("406 ms", OK), ("1053 ms", AMBAR)]
    cx = 830
    for label, col in chans:
        a(f'<rect x="{cx}" y="366" width="118" height="36" rx="10" fill="{CARBON}" stroke="{LINEA}"/>')
        a(f'<circle cx="{cx + 18}" cy="384" r="5" fill="{col}"/>')
        d, _ = text(label, cx + 32, 389, 14, MONO, 500)
        a(f'<path d="{d}" fill="{HUESO}"/>')
        cx += 130

    a("</g>")
    a(f'<rect x="1" y="1" width="{W - 2}" height="{H - 2}" rx="23" fill="none" stroke="{LINEA}" stroke-width="2"/>')
    a("</svg>")
    return "\n".join(out)


DIAGRAMA = {
    "en": {
        "label": "How Open TV works: the browser plays video directly from broadcasters; open-tv, on 127.0.0.1, syncs the lists and guides you add by URL, health-checks streams, stores everything in a local SQLite and relays a stream only when the browser can't fetch it.",
        "local": "YOUR MACHINE", "net": "INTERNET",
        "browser": ("Your browser", "Open TV web app"),
        "app": ("open-tv", "listens on 127.0.0.1 only"),
        "db": ("SQLite", "sources · catalog · stream health"),
        "streams": ("Broadcasters", "video streams · channel logos"),
        "lists": ("M3U lists by URL", "and their EPG guides"),
        "api": ("iptv-org API", "extra mirrors · iptv-org lists only"),
        "video": "video, directly", "ui": "UI · catalog · playback results",
        "proxy": "fallback proxy", "checks": "health checks · relays when needed",
        "sync": "syncs", "mirrors": "mirrors",
    },
    "es": {
        "label": "Cómo funciona Open TV: el navegador reproduce el vídeo directo de las emisoras; open-tv, en 127.0.0.1, sincroniza tus listas y guías, comprueba los streams, lo guarda todo en un SQLite local y solo retransmite un stream cuando el navegador no puede pedirlo.",
        "local": "TU MÁQUINA", "net": "INTERNET",
        "browser": ("Tu navegador", "app web de Open TV"),
        "app": ("open-tv", "escucha solo en 127.0.0.1"),
        "db": ("SQLite", "fuentes · catálogo · salud"),
        "streams": ("Emisoras", "streams de vídeo · logos"),
        "lists": ("Listas M3U por URL", "y sus guías EPG"),
        "api": ("API de iptv-org", "mirrors extra · solo sus listas"),
        "video": "vídeo, directo", "ui": "interfaz · catálogo · resultados",
        "proxy": "proxy de reserva", "checks": "comprobaciones · retransmite si hace falta",
        "sync": "sincroniza", "mirrors": "mirrors",
    },
}


def diagrama(lang):
    c = DIAGRAMA[lang]
    W, H = 1280, 600
    out = []
    a = out.append
    a(f'<svg xmlns="http://www.w3.org/2000/svg" width="{W}" height="{H}" viewBox="0 0 {W} {H}" role="img" aria-label="{c["label"]}">')
    a(f"<title>{c['label']}</title>")
    a("""<style>
.flow{stroke-dasharray:10 8;animation:flow 1.2s linear infinite}
.flow-slow{stroke-dasharray:4 7;animation:flow 2.4s linear infinite}
@keyframes flow{to{stroke-dashoffset:-36}}
@media (prefers-reduced-motion:reduce){*{animation:none!important}}
</style>""")
    a(f"""<defs>
<marker id="pa" viewBox="0 0 10 10" refX="8" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse"><path d="M0 0L10 5L0 10z" fill="{AMBAR}"/></marker>
<marker id="ps" viewBox="0 0 10 10" refX="8" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse"><path d="M0 0L10 5L0 10z" fill="{ACERO}"/></marker>
<clipPath id="card"><rect width="{W}" height="{H}" rx="24"/></clipPath>
<pattern id="dots" width="22" height="22" patternUnits="userSpaceOnUse"><circle cx="1" cy="1" r="1" fill="{LINEA}"/></pattern>
</defs>""")
    a('<g clip-path="url(#card)">')
    a(f'<rect width="{W}" height="{H}" fill="{GRAFITO}"/>')
    a(f'<rect width="{W}" height="{H}" fill="url(#dots)" opacity=".5"/>')

    def panel(x, y, w, h, titulo):
        a(f'<rect x="{x}" y="{y}" width="{w}" height="{h}" rx="18" fill="{CARBON}" fill-opacity=".55" stroke="{LINEA}" stroke-width="2"/>')
        d, _ = text(titulo, x + 24, y + 34, 14, MONO, 600, tracking=0.2)
        a(f'<path d="{d}" fill="{ACERO}"/>')

    def caja(cx, cy, w, h, t1, t2, destacada=False, cilindro=False):
        x, y = cx - w / 2, cy - h / 2
        borde = AMBAR if destacada else ACERO
        if cilindro:
            a(f'<path d="M{x} {y + 12} v{h - 24} a{w / 2} 12 0 0 0 {w} 0 v{-(h - 24)}" fill="{GRAFITO}" stroke="{borde}" stroke-width="2"/>')
            a(f'<ellipse cx="{cx}" cy="{y + 12}" rx="{w / 2}" ry="12" fill="{CARBON}" stroke="{borde}" stroke-width="2"/>')
            ty = cy + 8
        else:
            a(f'<rect x="{x}" y="{y}" width="{w}" height="{h}" rx="14" fill="{GRAFITO}" stroke="{borde}" stroke-width="{2.5 if destacada else 1.5}"/>')
            ty = cy
        w1 = width(t1, 21, SG, 600)
        d, _ = text(t1, cx - w1 / 2, ty - 2, 21, SG, 600)
        a(f'<path d="{d}" fill="{HUESO}"/>')
        w2 = width(t2, 13.5, MONO, 400)
        d, _ = text(t2, cx - w2 / 2, ty + 22, 13.5, MONO, 400)
        a(f'<path d="{d}" fill="{ACERO}"/>')

    def etiqueta(s, cx, cy, color=HUESO):
        w = width(s, 13.5, MONO, 500)
        a(f'<rect x="{cx - w / 2 - 10:.1f}" y="{cy - 14}" width="{w + 20:.1f}" height="26" rx="13" fill="{GRAFITO}" stroke="{LINEA}"/>')
        d, _ = text(s, cx - w / 2, cy + 4, 13.5, MONO, 500)
        a(f'<path d="{d}" fill="{color}"/>')

    # paneles
    panel(40, 40, 520, 520, c["local"])
    panel(720, 40, 520, 520, c["net"])

    # cajas: máquina
    BX, BY = 300, 150   # navegador
    OX, OY = 300, 340   # open-tv
    DX, DY = 300, 485   # sqlite
    # internet
    SX, SY = 980, 150   # emisoras
    MX, MY = 980, 340   # listas
    IX, IY = 980, 485   # iptv-org

    # flechas (debajo de las cajas)
    # vídeo directo, grueso y animado
    a(f'<path class="flow" d="M{BX + 150} {BY} H{SX - 160}" stroke="{AMBAR}" stroke-width="4" fill="none" marker-end="url(#pa)"/>')
    # navegador <-> open-tv
    a(f'<path d="M{BX - 40} {BY + 45} V{OY - 45}" stroke="{ACERO}" stroke-width="2" fill="none" marker-start="url(#ps)" marker-end="url(#ps)"/>')
    a(f'<path class="flow-slow" d="M{BX + 40} {BY + 45} V{OY - 45}" stroke="{AMBAR}" stroke-opacity=".8" stroke-width="2" fill="none" marker-end="url(#pa)"/>')
    # open-tv -> sqlite
    a(f'<path d="M{OX} {OY + 45} V{DY - 42}" stroke="{ACERO}" stroke-width="2" fill="none" marker-start="url(#ps)" marker-end="url(#ps)"/>')
    # open-tv -> emisoras (diagonal: comprobaciones y relevo)
    a(f'<path class="flow-slow" d="M{OX + 150} {OY - 20} C {OX + 330} {OY - 20}, {SX - 330} {SY + 40}, {SX - 160} {SY + 40}" stroke="{ACERO}" stroke-width="2" fill="none" marker-end="url(#ps)"/>')
    # open-tv -> listas
    a(f'<path d="M{OX + 150} {OY + 10} H{MX - 160}" stroke="{ACERO}" stroke-width="2" fill="none" marker-end="url(#ps)"/>')
    # open-tv -> iptv-org
    a(f'<path class="flow-slow" d="M{OX + 150} {OY + 35} C {OX + 330} {OY + 35}, {IX - 330} {IY}, {IX - 160} {IY}" stroke="{ACERO}" stroke-width="2" fill="none" marker-end="url(#ps)"/>')

    caja(BX, BY, 300, 90, *c["browser"])
    caja(OX, OY, 300, 90, *c["app"], destacada=True)
    caja(DX, DY, 300, 84, *c["db"], cilindro=True)
    caja(SX, SY, 320, 90, *c["streams"])
    caja(MX, MY, 320, 90, *c["lists"])
    caja(IX, IY, 320, 84, *c["api"])

    etiqueta(c["video"], (BX + SX) / 2, BY - 26, AMBAR)
    etiqueta(c["ui"], BX - 100, (BY + OY) / 2 - 16)
    etiqueta(c["proxy"], BX + 130, (BY + OY) / 2 + 22, AMBAR)
    etiqueta(c["checks"], (OX + SX) / 2, (OY + SY) / 2 - 6)
    etiqueta(c["sync"], (OX + MX) / 2, OY + 10)
    etiqueta(c["mirrors"], (OX + IX) / 2, (OY + IY) / 2 + 30)

    a("</g>")
    a(f'<rect x="1" y="1" width="{W - 2}" height="{H - 2}" rx="23" fill="none" stroke="{LINEA}" stroke-width="2"/>')
    a("</svg>")
    return "\n".join(out)


if __name__ == "__main__":
    dest = os.path.join(RAIZ, "assets", "readme")
    for lang in ("es", "en"):
        with open(f"{dest}/banner-{lang}.svg", "w") as fh:
            fh.write(build(lang) + "\n")
        with open(f"{dest}/diagrama-{lang}.svg", "w") as fh:
            fh.write(diagrama(lang) + "\n")
