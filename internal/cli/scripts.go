package cli

import (
	"fmt"
	"strings"
)

// ScriptVPSSetup — установка Crystal-Core VPS Install Manager на сервер.
const ScriptVPSSetup = "vps-setup"

// VPSSetupURL — последняя версия установщика; архитектуру подставляет dpkg.
const VPSSetupURL = "https://github.com/itmaxk/crystal-core-vps-install-manager/releases/latest/download/vps-setup-linux-$(dpkg --print-architecture)"

// scripts — фиксированный набор команд, которые можно выполнить на сервере.
// Из GUI и командной строки передаётся только имя: произвольный текст
// команды через аргументы терминала не идёт.
var scripts = map[string]string{
	ScriptVPSSetup: `cd ~ || exit 1
if ! command -v dpkg >/dev/null 2>&1; then
  echo "vps-setup: нужен Ubuntu или Debian (dpkg не найден)"
else
  url="` + VPSSetupURL + `"
  echo "+ curl -fsSLo vps-setup $url"
  if curl -fsSLo vps-setup "$url" && chmod +x vps-setup; then
    if [ "$(id -u)" -eq 0 ]; then ./vps-setup; else sudo ./vps-setup; fi
  else
    echo "vps-setup: не удалось скачать установщик"
  fi
fi
echo
echo "Установщик завершил работу. Это обычная SSH-сессия; закройте вкладку или введите exit."
exec "${SHELL:-/bin/sh}" -l`,
}

// IsScript сообщает, есть ли скрипт с таким именем.
func IsScript(name string) bool {
	_, ok := scripts[name]
	return ok
}

// remoteCommand оборачивает скрипт в sh -c: у пользователя на сервере может
// быть не POSIX-оболочка (fish), а ssh передаёт команду именно ей.
func remoteCommand(name string) (string, error) {
	body, ok := scripts[name]
	if !ok {
		return "", fmt.Errorf("unknown script %q", name)
	}
	return "sh -c " + shellQuote(body), nil
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}
