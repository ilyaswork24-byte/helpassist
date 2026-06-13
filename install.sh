#!/usr/bin/env sh
# Установщик helpassist для любого устройства Ubuntu/Debian (amd64/arm64/arm).
# Использование (одной строкой):
#   curl -fsSL https://raw.githubusercontent.com/ilyaswork24-byte/helpassist/main/install.sh | sh
#
# Логика:
#   1. Если установлен Go — собирает из исходников под вашу архитектуру (надёжнее всего).
#   2. Иначе — скачивает готовый бинарник из последнего GitHub Release.
#   3. Ставит в ~/.local/bin и проверяет наличие Ollama и модели qwen3.
set -eu

REPO="ilyaswork24-byte/helpassist"
BINARY="helpassist"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
BRANCH="${BRANCH:-main}"

info() { printf '\033[36m==>\033[0m %s\n' "$1"; }
warn() { printf '\033[33mвнимание:\033[0m %s\n' "$1" >&2; }
die()  { printf '\033[31mошибка:\033[0m %s\n' "$1" >&2; exit 1; }

# --- определение архитектуры ---
os="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$os" in
  linux) os=linux ;;
  darwin) os=darwin ;;
  *) die "неподдерживаемая ОС: $os (нужен Linux или macOS)" ;;
esac

arch="$(uname -m)"
case "$arch" in
  x86_64|amd64) arch=amd64 ;;
  aarch64|arm64) arch=arm64 ;;
  armv7l|armv6l|arm) arch=arm ;;
  *) die "неподдерживаемая архитектура: $arch" ;;
esac
info "Платформа: ${os}/${arch}"

mkdir -p "$INSTALL_DIR"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

if command -v go >/dev/null 2>&1; then
  info "Найден Go ($(go version | awk '{print $3}')) — собираю из исходников."
  if [ -f "./go.mod" ] && grep -q '^module helpassist' ./go.mod 2>/dev/null; then
    src="."
  else
    info "Скачиваю исходники ветки ${BRANCH}..."
    tarball="https://codeload.github.com/${REPO}/tar.gz/refs/heads/${BRANCH}"
    if command -v curl >/dev/null 2>&1; then
      curl -fsSL "$tarball" -o "$tmp/src.tgz"
    else
      wget -qO "$tmp/src.tgz" "$tarball"
    fi
    tar -xzf "$tmp/src.tgz" -C "$tmp"
    src="$(find "$tmp" -maxdepth 1 -type d -name 'helpassist-*')"
    [ -n "$src" ] || die "не удалось распаковать исходники"
  fi
  ( cd "$src" && go build -ldflags "-s -w" -o "$tmp/$BINARY" . )
  install -m 0755 "$tmp/$BINARY" "$INSTALL_DIR/$BINARY"
else
  info "Go не найден — скачиваю готовый бинарник из релиза."
  asset="${BINARY}-${os}-${arch}"
  url="https://github.com/${REPO}/releases/latest/download/${asset}"
  info "Загрузка $url"
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$url" -o "$tmp/$BINARY" || die "не удалось скачать бинарник (есть ли релиз для ${os}/${arch}?). Установите Go и повторите."
  else
    wget -qO "$tmp/$BINARY" "$url" || die "не удалось скачать бинарник. Установите Go и повторите."
  fi
  install -m 0755 "$tmp/$BINARY" "$INSTALL_DIR/$BINARY"
fi

info "Установлено: $INSTALL_DIR/$BINARY"

# --- PATH ---
case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *) warn "$INSTALL_DIR не в PATH. Добавьте в ~/.bashrc или ~/.profile:"
     printf '    export PATH="%s:$PATH"\n' "$INSTALL_DIR" ;;
esac

# --- проверка Ollama / qwen3 ---
if command -v ollama >/dev/null 2>&1; then
  if ollama list 2>/dev/null | grep -qi 'qwen3'; then
    info "Ollama и модель qwen3 найдены — всё готово."
  else
    warn "Ollama есть, но модель qwen3 не загружена. Выполните: ollama pull qwen3:8b"
  fi
else
  warn "Ollama не установлен. Установите: curl -fsSL https://ollama.com/install.sh | sh"
  warn "Затем: ollama pull qwen3:8b"
fi

info "Запуск: $BINARY   (или $INSTALL_DIR/$BINARY)"
