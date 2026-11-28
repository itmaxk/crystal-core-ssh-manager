#!/usr/bin/env bash
# Устанавливает Crystal-Core SSH Manager для текущего пользователя Linux:
#   * бинарник в ~/.local/bin;
#   * ярлык в меню приложений и иконку;
#   * автозапуск при входе в систему;
#   * значок на панели XFCE (в док, если он есть).
#
# Повторный запуск не создаёт дублей. Флаги:
#   --no-panel        не добавлять на панель XFCE
#   --no-autostart    не включать автозапуск
#   --binary <path>   установить указанный бинарник вместо build/bin/...
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
APP_ID="crystal-core-ssh-manager"
DESKTOP_ID="${APP_ID}.desktop"
BIN_DIR="$HOME/.local/bin"
BIN="${BIN_DIR}/CrystalCoreSSHManager"
APPS_DIR="$HOME/.local/share/applications"
ICON_DIR="$HOME/.local/share/icons/hicolor/512x512/apps"
AUTOSTART_DIR="$HOME/.config/autostart"
PANEL_DIR="$HOME/.config/xfce4/panel"
SOURCE_BIN="${ROOT}/build/bin/CrystalCoreSSHManager"
WITH_PANEL=1
WITH_AUTOSTART=1

log()  { printf '\n\033[1;36m==> %s\033[0m\n' "$*"; }
warn() { printf '\033[1;33mWARN: %s\033[0m\n' "$*"; }
die()  { printf '\033[1;31mERROR: %s\033[0m\n' "$*" >&2; exit 1; }

usage() {
	sed -n '2,14p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
}

while [ $# -gt 0 ]; do
	case "$1" in
		--no-panel)     WITH_PANEL=0 ;;
		--no-autostart) WITH_AUTOSTART=0 ;;
		--binary)       [ $# -ge 2 ] || die "--binary требует путь"; SOURCE_BIN="$2"; shift ;;
		-h|--help)      usage; exit 0 ;;
		*)              die "неизвестный флаг: $1 (см. --help)" ;;
	esac
	shift
done

[ "$(uname -s)" = Linux ] || die "скрипт только для Linux; на macOS/Windows установите приложение вручную"

# --- 1. Бинарник -------------------------------------------------------------

detect_webkit_tag() {
	if pkg-config --exists webkit2gtk-4.1 2>/dev/null; then
		echo webkit2_41
	elif pkg-config --exists webkit2gtk-4.0 2>/dev/null; then
		echo webkit2_40
	fi
}

if [ ! -x "$SOURCE_BIN" ]; then
	log "Бинарник не найден, собираю"
	command -v wails >/dev/null 2>&1 || die "wails не найден — сначала выполните ./scripts/setup-dev.sh"
	TAG="$(detect_webkit_tag)"
	if [ -n "$TAG" ]; then
		(cd "$ROOT" && wails build -clean -tags "$TAG")
	else
		(cd "$ROOT" && wails build -clean)
	fi
fi

log "Установка бинарника: $BIN"
install -Dm755 "$SOURCE_BIN" "$BIN"

install -Dm644 "$ROOT/build/appicon.png" "${ICON_DIR}/${APP_ID}.png"

# --- 2. Ярлык в меню ---------------------------------------------------------

EXEC_FIELD="$BIN"
case "$EXEC_FIELD" in *" "*) EXEC_FIELD="\"${EXEC_FIELD}\"" ;; esac

write_desktop_file() {
	cat > "$1" <<EOF
[Desktop Entry]
Version=1.0
Type=Application
Name=Crystal-Core SSH Manager
Comment=Lightweight SSH manager and launcher
Exec=${EXEC_FIELD}
TryExec=${BIN}
Icon=${APP_ID}
Terminal=false
Categories=Network;RemoteAccess;
StartupWMClass=CrystalCoreSSHManager
StartupNotify=false
$2
EOF
}

log "Ярлык в меню: ${APPS_DIR}/${DESKTOP_ID}"
mkdir -p "$APPS_DIR"
write_desktop_file "${APPS_DIR}/${DESKTOP_ID}" ""

# --- 3. Автозапуск -----------------------------------------------------------

if [ "$WITH_AUTOSTART" = 1 ]; then
	log "Автозапуск: ${AUTOSTART_DIR}/${DESKTOP_ID}"
	mkdir -p "$AUTOSTART_DIR"
	write_desktop_file "${AUTOSTART_DIR}/${DESKTOP_ID}" "X-GNOME-Autostart-enabled=true
X-GNOME-Autostart-Delay=3"
else
	warn "автозапуск пропущен (--no-autostart)"
fi

# --- 4. Панель XFCE ----------------------------------------------------------

panel_ids() {
	xfconf-query -c xfce4-panel -p "/panels/panel-$1/plugin-ids" 2>/dev/null | grep -E '^[0-9]+$' || true
}

pick_panel() {
	local best=1 best_size=0 p size
	for p in $(xfconf-query -c xfce4-panel -p /panels -l 2>/dev/null |
		sed -n 's#^/panels/panel-\([0-9]\+\)/.*#\1#p' | sort -nu); do
		size="$(xfconf-query -c xfce4-panel -p "/panels/panel-$p/size" 2>/dev/null || echo 0)"
		if [ "$size" -gt "$best_size" ]; then
			best="$p"
			best_size="$size"
		fi
	done
	echo "$best"
}

launcher_pid_for_app() {
	local ids pid
	ids="$(panel_ids "$1")"
	for pid in $ids; do
		[ "$(xfconf-query -c xfce4-panel -p "/plugins/plugin-$pid" 2>/dev/null || true)" = launcher ] || continue
		if xfconf-query -c xfce4-panel -p "/plugins/plugin-$pid/items" 2>/dev/null |
			grep -qxF "$DESKTOP_ID"; then
			echo "$pid"
			return 0
		fi
	done
	return 1
}

pin_to_xfce_panel() {
	command -v xfconf-query >/dev/null 2>&1 || { warn "xfconf-query не найден — панель XFCE не настроена"; return; }
	if ! { [ -n "${DISPLAY:-}" ] && [ -n "${DBUS_SESSION_BUS_ADDRESS:-}" ]; }; then
		warn "нет графической сессии (DISPLAY/DBUS) — добавьте значок на панель вручную"
		return
	fi
	xfconf-query -c xfce4-panel -l >/dev/null 2>&1 || {
		warn "канал xfce4-panel недоступен — панель не настроена"
		return
	}

	local panel pid_ids existing pid max new_id was_running=0
	panel="$(pick_panel)"
	if pid_ids="$(launcher_pid_for_app "$panel")"; then
		mkdir -p "${PANEL_DIR}/launcher-${pid_ids}"
		cp "${APPS_DIR}/${DESKTOP_ID}" "${PANEL_DIR}/launcher-${pid_ids}/${DESKTOP_ID}"
		log "Панель XFCE: значок уже закреплён (plugin-${pid_ids})"
		return
	fi

	log "Панель XFCE: добавление значка в panel-${panel}"
	if pgrep -x xfce4-panel >/dev/null 2>&1; then
		was_running=1
		# Останавливаем панель, иначе при выходе она перезапишет правки xfconf.
		xfce4-panel -q >/dev/null 2>&1 || true
		sleep 1
	fi

	existing="$(panel_ids "$panel")"
	max="$(xfconf-query -c xfce4-panel -l 2>/dev/null |
		sed -n 's#^/plugins/plugin-\([0-9]\+\)$#\1#p' | sort -n | tail -n1)"
	max="${max:-0}"
	new_id=$((max + 1))

	xfconf-query -c xfce4-panel -p "/plugins/plugin-${new_id}" -n -t string -s launcher
	xfconf-query -c xfce4-panel -p "/plugins/plugin-${new_id}/items" -n -a \
		-t string -s "$DESKTOP_ID"
	mkdir -p "${PANEL_DIR}/launcher-${new_id}"
	cp "${APPS_DIR}/${DESKTOP_ID}" "${PANEL_DIR}/launcher-${new_id}/${DESKTOP_ID}"

	local -a args=()
	for pid in $existing; do
		args+=(-t int -s "$pid")
	done
	args+=(-t int -s "$new_id")
	if [ -z "$existing" ]; then
		xfconf-query -c xfce4-panel -p "/panels/panel-${panel}/plugin-ids" -n -a "${args[@]}"
	else
		xfconf-query -c xfce4-panel -p "/panels/panel-${panel}/plugin-ids" -a "${args[@]}"
	fi

	# Даём xfconf сохранить конфиг на диск до старта панели.
	sleep 2
	if [ "$was_running" = 1 ]; then
		nohup xfce4-panel --display "$DISPLAY" >/dev/null 2>&1 &
		disown 2>/dev/null || true
		sleep 2
	fi
	log "Панель XFCE: значок добавлен (plugin-${new_id})"
}

if [ "$WITH_PANEL" = 1 ]; then
	pin_to_xfce_panel
else
	warn "панель пропущена (--no-panel)"
fi

# --- 5. Кеши ----------------------------------------------------------------

if command -v update-desktop-database >/dev/null 2>&1; then
	update-desktop-database "$APPS_DIR" >/dev/null 2>&1 || true
fi
if command -v gtk-update-icon-cache >/dev/null 2>&1; then
	gtk-update-icon-cache -q -t "$HOME/.local/share/icons/hicolor" >/dev/null 2>&1 || true
fi

log "Готово"
echo "  Приложение: $BIN"
echo "  Меню:       ${APPS_DIR}/${DESKTOP_ID}"
if [ "$WITH_AUTOSTART" = 1 ]; then
	echo "  Автозапуск: ${AUTOSTART_DIR}/${DESKTOP_ID}"
	echo "  Удалить автозапуск: rm ${AUTOSTART_DIR}/${DESKTOP_ID}"
fi
