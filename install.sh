#!/bin/sh
# install.sh — instalador de Korven Open TV.
#
# Uso documentado en el README:
#   curl -fsSL https://raw.githubusercontent.com/gdberysan/open-tv/main/install.sh | sh
#
# POSIX sh a propósito (se ejecuta con `sh`, no `bash`): nada de arrays,
# `[[ ]]`, `local` ni otros bashismos. Descarga el binario de la última
# release de GitHub para el sistema/arquitectura detectados, verifica su
# checksum contra checksums.txt ANTES de instalar nada, y lo deja en
# ~/.local/bin o /usr/local/bin.
set -eu

repo="gdberysan/open-tv"
api_url="https://api.github.com/repos/${repo}/releases/latest"

# --- utilidades ---------------------------------------------------------

err() {
    printf 'error: %s\n' "$1" >&2
    exit 1
}

info() {
    printf '==> %s\n' "$1"
}

necesita() {
    command -v "$1" >/dev/null 2>&1 || err "hace falta '$1' y no está en el PATH."
}

# --- comprobaciones previas ----------------------------------------------

necesita curl
necesita tar

sistema_checksum=""
if command -v sha256sum >/dev/null 2>&1; then
    sistema_checksum="sha256sum"
elif command -v shasum >/dev/null 2>&1; then
    sistema_checksum="shasum -a 256"
else
    err "hace falta 'sha256sum' o 'shasum' para verificar el checksum y ninguno está en el PATH."
fi

# --- detección de sistema y arquitectura --------------------------------

so_crudo=$(uname -s)
case "$so_crudo" in
    Darwin) so="darwin" ;;
    Linux) so="linux" ;;
    *)
        err "sistema '${so_crudo}' no soportado por este instalador (solo macOS y Linux). En Windows descarga el asset de https://github.com/${repo}/releases/latest a mano."
        ;;
esac

arch_crudo=$(uname -m)
case "$arch_crudo" in
    x86_64 | amd64) arch="amd64" ;;
    arm64 | aarch64) arch="arm64" ;;
    *)
        err "arquitectura '${arch_crudo}' no soportada por este instalador. Compílalo desde el código fuente (ver el README) o usa la imagen de Docker."
        ;;
esac

info "Sistema detectado: ${so}/${arch}"

# --- resolver la última release -----------------------------------------

info "Consultando la última release de ${repo}..."
release_json=$(curl -fsSL "$api_url") ||
    err "no se pudo consultar ${api_url}. ¿Hay conexión? ¿Existe ya una release publicada?"

etiqueta=$(printf '%s\n' "$release_json" |
    grep -m1 '"tag_name"' |
    sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')
[ -n "$etiqueta" ] || err "no se pudo leer 'tag_name' de la respuesta de GitHub. ¿Existe alguna release publicada en https://github.com/${repo}/releases?"

info "Última release: ${etiqueta}"

urls_assets=$(printf '%s\n' "$release_json" |
    grep -o '"browser_download_url": *"[^"]*"' |
    sed -E 's/.*"([^"]+)"$/\1/')

url_archivo=$(printf '%s\n' "$urls_assets" | grep "_${so}_${arch}\.tar\.gz$" | head -n1 || true)
[ -n "$url_archivo" ] || err "no hay ningún asset para ${so}/${arch} en la release ${etiqueta}."

url_checksums=$(printf '%s\n' "$urls_assets" | grep '/checksums\.txt$' | head -n1 || true)
[ -n "$url_checksums" ] || err "la release ${etiqueta} no trae 'checksums.txt'; no se puede verificar el binario, así que no se instala."

nombre_archivo=$(basename "$url_archivo")

# --- descarga y verificación ---------------------------------------------

dir_tmp=$(mktemp -d "${TMPDIR:-/tmp}/open-tv-install.XXXXXX")
limpiar() {
    rm -rf "$dir_tmp"
}
trap limpiar EXIT INT TERM

info "Descargando ${nombre_archivo}..."
curl -fsSL -o "${dir_tmp}/${nombre_archivo}" "$url_archivo" ||
    err "no se pudo descargar ${url_archivo}."

info "Descargando checksums.txt..."
curl -fsSL -o "${dir_tmp}/checksums.txt" "$url_checksums" ||
    err "no se pudo descargar ${url_checksums}."

info "Verificando el checksum..."
checksum_esperado=$(grep " ${nombre_archivo}\$" "${dir_tmp}/checksums.txt" | awk '{print $1}')
[ -n "$checksum_esperado" ] || err "'${nombre_archivo}' no aparece en checksums.txt; no se instala nada sin poder verificarlo."

checksum_real=$(cd "$dir_tmp" && $sistema_checksum "$nombre_archivo" | awk '{print $1}')

if [ "$checksum_esperado" != "$checksum_real" ]; then
    err "el checksum de ${nombre_archivo} no coincide (esperado ${checksum_esperado}, obtenido ${checksum_real}). Instalación abortada."
fi
info "Checksum verificado."

# --- extracción ------------------------------------------------------------

tar -xzf "${dir_tmp}/${nombre_archivo}" -C "$dir_tmp" ||
    err "no se pudo extraer ${nombre_archivo}."

[ -f "${dir_tmp}/open-tv" ] || err "el archivo descargado no contiene el binario 'open-tv' esperado."

# --- elegir destino e instalar ---------------------------------------------

if [ -w /usr/local/bin ]; then
    destino_dir="/usr/local/bin"
else
    destino_dir="${HOME}/.local/bin"
    mkdir -p "$destino_dir"
fi

destino="${destino_dir}/open-tv"
cp "${dir_tmp}/open-tv" "$destino"
chmod +x "$destino"

info "Instalado en ${destino}"

case ":${PATH}:" in
    *":${destino_dir}:"*) ;;
    *)
        printf '\n'
        info "AVISO: '${destino_dir}' no está en tu PATH."
        printf '    Añade esta línea a tu shell (~/.zshrc, ~/.bashrc, ...):\n'
        # shellcheck disable=SC2016 # el $PATH del mensaje es literal, para
        # que el usuario lo copie tal cual a su fichero de shell.
        printf '        export PATH="%s:$PATH"\n\n' "$destino_dir"
        ;;
esac

# --- arranque ---------------------------------------------------------------

if command -v open-tv >/dev/null 2>&1; then
    info "Arrancando Open TV..."
    exec open-tv
else
    info "Arráncalo con:"
    printf '    %s\n' "$destino"
fi
