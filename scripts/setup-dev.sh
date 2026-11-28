#!/usr/bin/env bash
# Быстрая установка окружения для сборки Crystal-Core SSH Manager.
#
# Скрипт сначала определяет ОС и архитектуру (amd64/arm64), затем ставит
# подходящие Go, Wails CLI и системные библиотеки GTK/WebKit. Повторный
# запуск безопасен: уже установленные компоненты пропускаются.
#
# Использование:
#   ./scripts/setup-dev.sh
#   GO_VERSION=go1.27.4 ./scripts/setup-dev.sh   # закрепить версию Go
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GO_REQUIRED="$(awk '$1 == "go" { print $2; exit }' "$ROOT/go.mod")"
WAILS_VERSION="$(awk '$1 == "github.com/wailsapp/wails/v2" { print $2; exit }' "$ROOT/go.mod")"
WAILS_VERSION="${WAILS_VERSION:-v2.17.1}"
WEBKIT_TAG=""

log()  { printf '\n\033[1;36m==> %s\033[0m\n' "$*"; }
warn() { printf '\033[1;33mWARN: %s\033[0m\n' "$*"; }
die()  { printf '\033[1;31mERROR: %s\033[0m\n' "$*" >&2; exit 1; }

run_root() {
	if [ "$(id -u)" -eq 0 ]; then
		"$@"
	elif command -v sudo >/dev/null 2>&1; then
		sudo "$@"
	else
		die "нужны права root для: $*"
	fi
}

# --- 1. Определяем ОС и архитектуру -----------------------------------------

case "$(uname -s)" in
	Linux)  OS=linux ;;
	Darwin) OS=darwin ;;
	*) die "неподдерживаемая ОС: $(uname -s). Соберите вручную по README." ;;
esac

case "$(uname -m)" in
	x86_64|amd64)   ARCH=amd64 ;;
	aarch64|arm64)  ARCH=arm64 ;;
	armv7l|armv6l)  die "32-битный ARM не поддерживается: нужен aarch64 (arm64)" ;;
	*)              die "неподдерживаемая архитектура: $(uname -m)" ;;
esac

log "Платформа: ${OS}/${ARCH}"

# --- 2. Системные зависимости и тег WebKit ----------------------------------

install_linux_deps() {
	local pm
	if command -v apt-get >/dev/null 2>&1; then pm=apt
	elif command -v dnf >/dev/null 2>&1; then pm=dnf
	elif command -v pacman >/dev/null 2>&1; then pm=pacman
	elif command -v zypper >/dev/null 2>&1; then pm=zypper
	else die "не найден пакетный менеджер (apt/dnf/pacman/zypper)"
	fi

	case "$pm" in
		apt)
			run_root apt-get update -qq
			local pkgs=(build-essential pkg-config libgtk-3-dev libayatana-appindicator3-dev)
			if apt-cache show -qq libwebkit2gtk-4.1-dev >/dev/null 2>&1; then
				pkgs+=(libwebkit2gtk-4.1-dev)
				WEBKIT_TAG=webkit2_41
			elif apt-cache show -qq libwebkit2gtk-4.0-dev >/dev/null 2>&1; then
				pkgs+=(libwebkit2gtk-4.0-dev)
				WEBKIT_TAG=webkit2_40
			else
				die "в репозиториях нет libwebkit2gtk-4.1-dev или libwebkit2gtk-4.0-dev"
			fi
			run_root env DEBIAN_FRONTEND=noninteractive apt-get install -y -qq "${pkgs[@]}"
			;;
		dnf)
			local pkgs=(gcc gcc-c++ make pkgconf-pkg-config gtk3-devel libappindicator-gtk3-devel)
			if dnf -q list --available webkit2gtk4.1-devel >/dev/null 2>&1; then
				pkgs+=(webkit2gtk4.1-devel)
				WEBKIT_TAG=webkit2_41
			else
				pkgs+=(webkit2gtk4.0-devel)
				WEBKIT_TAG=webkit2_40
			fi
			run_root dnf install -y "${pkgs[@]}"
			;;
		pacman)
			run_root pacman -Sy --needed --noconfirm base-devel pkgconf gtk3 webkit2gtk-4.1 libayatana-appindicator
			WEBKIT_TAG=webkit2_41
			;;
		zypper)
			run_root zypper --non-interactive install gcc gcc-c++ make pkg-config gtk3-devel webkit2gtk4.1-devel libayatana-appindicator3-devel
			WEBKIT_TAG=webkit2_41
			;;
	esac
}

if [ "$OS" = linux ]; then
	log "Системные зависимости (GTK / WebKitGTK / appindicator)"
	install_linux_deps
	log "WebKitGTK: ${WEBKIT_TAG}"
else
	log "Системные зависимости macOS (Xcode Command Line Tools)"
	if ! xcode-select -p >/dev/null 2>&1; then
		xcode-select --install || true
		warn "дождитесь установки Command Line Tools и запустите скрипт ещё раз"
	fi
fi

# --- 3. Go нужной версии -----------------------------------------------------

version_ge() { [ "$(printf '%s\n%s\n' "$1" "$2" | sort -V | head -n1)" = "$2" ]; }

go_version() { "$1" env GOVERSION 2>/dev/null | sed 's/^go//'; }

GO_BIN="$(command -v go 2>/dev/null || true)"
if [ -n "$GO_BIN" ] && version_ge "$(go_version "$GO_BIN")" "$GO_REQUIRED"; then
	log "Go уже установлен: $("$GO_BIN" version)"
else
	log "Установка Go (требуется >= ${GO_REQUIRED})"
	GO_VERSION="${GO_VERSION:-$(curl -fsSL 'https://go.dev/VERSION?m=text' | head -n1)}"
	case "$GO_VERSION" in go*) ;; *) GO_VERSION="go${GO_VERSION}" ;; esac
	ARCHIVE="${GO_VERSION}.${OS}-${ARCH}.tar.gz"
	TMPDIR_GO="$(mktemp -d)"
	trap 'rm -rf "$TMPDIR_GO"' EXIT
	curl -fL --retry 3 -o "${TMPDIR_GO}/${ARCHIVE}" "https://go.dev/dl/${ARCHIVE}"
	if [ -w /usr/local ] || [ "$(id -u)" -eq 0 ]; then
		run_root rm -rf /usr/local/go
		run_root tar -C /usr/local -xzf "${TMPDIR_GO}/${ARCHIVE}"
		GO_BIN=/usr/local/go/bin/go
	else
		mkdir -p "$HOME/.local"
		rm -rf "$HOME/.local/go"
		tar -C "$HOME/.local" -xzf "${TMPDIR_GO}/${ARCHIVE}"
		GO_BIN="$HOME/.local/go/bin/go"
	fi
	log "Установлен $("$GO_BIN" version)"
fi

GO_DIR="$(dirname "$GO_BIN")"
export PATH="${GO_DIR}:${PATH}"
GOBIN_DIR="$("$GO_BIN" env GOPATH)/bin"
export PATH="${GOBIN_DIR}:${PATH}"

# Прописываем PATH в профили, чтобы wails/go были доступны в новых терминалах.
PATH_MARKER="# Crystal-Core SSH Manager: Go / Wails"
PATH_LINE="export PATH=\"${GO_DIR}:\$HOME/go/bin:\$PATH\""
for rc in "$HOME/.bashrc" "$HOME/.profile" "$HOME/.zshrc"; do
	[ -f "$rc" ] || continue
	grep -qF "$PATH_MARKER" "$rc" && continue
	printf '\n%s\n%s\n' "$PATH_MARKER" "$PATH_LINE" >> "$rc"
done

# --- 4. Wails CLI ------------------------------------------------------------

installed_wails="$("$GO_BIN" env GOPATH)/bin/wails"
if [ -x "$installed_wails" ] && [ "$("$installed_wails" version 2>/dev/null | grep -o 'v[0-9][0-9.]*' | head -n1)" = "$WAILS_VERSION" ]; then
	log "Wails CLI уже установлен: ${WAILS_VERSION}"
else
	log "Установка Wails CLI ${WAILS_VERSION}"
	"$GO_BIN" install "github.com/wailsapp/wails/v2/cmd/wails@${WAILS_VERSION}"
fi

# --- 5. Node.js (только проверка) -------------------------------------------

if command -v node >/dev/null 2>&1; then
	node_major="$(node -v | sed 's/^v\([0-9]*\).*/\1/')"
	if [ "$node_major" -lt 22 ]; then
		warn "Node.js $(node -v) — нужен 22+. Обновите Node (nvm/fnm) перед сборкой фронтенда."
	else
		log "Node.js: $(node -v)"
	fi
else
	warn "Node.js не найден — установите 22+ (https://nodejs.org или nvm/fnm)."
fi

# --- 6. Итог ----------------------------------------------------------------

log "Готово: ${OS}/${ARCH}"
if [ -n "$WEBKIT_TAG" ]; then
	echo "  Сборка: wails build -clean -tags ${WEBKIT_TAG}"
else
	echo "  Сборка: wails build -clean"
fi
