//go:build linux

package app

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
)

func agentSetupSupported() bool { return true }

func setupLocalAgentUser(name string, closeHome bool) (AgentSetupResult, error) {
	current, err := user.Current()
	if err != nil {
		return AgentSetupResult{}, err
	}
	cfgDir, err := agentConfigDir()
	if err != nil {
		return AgentSetupResult{}, err
	}

	script := `#!/bin/sh
set -eu
USER_NAME="$1"
CLOSE_HOME="$2"
APP_UID="$3"
APP_GID="$4"
CFG_DIR="$5"
CURRENT_USER="$6"

case "$USER_NAME" in
  *[!a-z0-9_-]*|"") echo "invalid username" >&2; exit 2 ;;
esac
[ "$(id -u)" = 0 ] || { echo "root required" >&2; exit 3; }

AGENT_HOME="$(getent passwd "$USER_NAME" 2>/dev/null | cut -d: -f6 || true)"
if [ -n "$AGENT_HOME" ]; then
  if [ ! -f "$AGENT_HOME/.crystal-core-agent" ]; then
    echo "user $USER_NAME already exists and is not managed by Crystal-Core" >&2
    exit 4
  fi
  CREATED=0
else
  useradd --create-home --shell /bin/bash --user-group "$USER_NAME"
  AGENT_HOME="$(getent passwd "$USER_NAME" | cut -d: -f6)"
  CREATED=1
  : > "$AGENT_HOME/.crystal-core-agent"
  chown "$USER_NAME:$USER_NAME" "$AGENT_HOME/.crystal-core-agent"
  chmod 600 "$AGENT_HOME/.crystal-core-agent"
fi

# Никаких дополнительных групп, sudo и пароля: только собственный домашний каталог.
usermod -G "" "$USER_NAME" 2>/dev/null || true
usermod -p '!' "$USER_NAME" 2>/dev/null || true
for g in sudo adm docker lxd; do
  gpasswd -d "$USER_NAME" "$g" >/dev/null 2>&1 || true
done

install -d -m 700 -o "$USER_NAME" -g "$USER_NAME" "$AGENT_HOME/.ssh"
if [ ! -f "$AGENT_HOME/.ssh/id_ed25519" ]; then
  runuser -u "$USER_NAME" -- ssh-keygen -q -t ed25519 \
    -f "$AGENT_HOME/.ssh/id_ed25519" -N "" -C "$USER_NAME@crystal-core"
fi
chmod 700 "$AGENT_HOME" "$AGENT_HOME/.ssh"
chmod 600 "$AGENT_HOME/.ssh/id_ed25519" "$AGENT_HOME/.ssh/id_ed25519.pub"
chown -R "$USER_NAME:$USER_NAME" "$AGENT_HOME/.ssh"

if [ "$CLOSE_HOME" = 1 ]; then
  CURRENT_HOME="$(getent passwd "$CURRENT_USER" 2>/dev/null | cut -d: -f6 || true)"
  if [ -n "$CURRENT_HOME" ] && [ "$CURRENT_HOME" != "$AGENT_HOME" ]; then
    chmod 750 "$CURRENT_HOME" || true
  fi
fi

PUB="$(cat "$AGENT_HOME/.ssh/id_ed25519.pub")"
mkdir -p "$CFG_DIR"
printf '{"username":"%s","home":"%s","public_key":"%s","created_at":"%s"}\n' \
  "$USER_NAME" "$AGENT_HOME" "$PUB" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  > "$CFG_DIR/agent-setup.json"
chown "$APP_UID:$APP_GID" "$CFG_DIR/agent-setup.json"
chmod 600 "$CFG_DIR/agent-setup.json"
echo "AGENT_USER=$USER_NAME"
echo "AGENT_HOME=$AGENT_HOME"
echo "AGENT_CREATED=$CREATED"
`

	dir, err := os.MkdirTemp("", "cc-agent-setup-")
	if err != nil {
		return AgentSetupResult{}, err
	}
	scriptPath := filepath.Join(dir, "setup-agent.sh")
	if err := os.WriteFile(scriptPath, []byte(script), 0700); err != nil {
		return AgentSetupResult{}, err
	}

	args := []string{
		"/bin/sh", scriptPath,
		name,
		boolArg(closeHome),
		strconv.Itoa(os.Getuid()),
		strconv.Itoa(os.Getgid()),
		cfgDir,
		current.Username,
	}
	displayArgs := make([]string, len(args))
	for i, arg := range args {
		displayArgs[i] = shQuote(arg)
	}
	manual := "sudo " + strings.Join(displayArgs, " ")

	run := func(program string, cmdArgs []string) (string, int, error) {
		cmd := exec.Command(program, cmdArgs...)
		out, err := cmd.CombinedOutput()
		code := 0
		if err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				code = exitErr.ExitCode()
			} else {
				return string(out), -1, err
			}
		}
		return string(out), code, nil
	}

	if os.Geteuid() == 0 {
		out, code, err := run("/bin/sh", args[1:])
		if err != nil || code != 0 {
			return AgentSetupResult{}, fmt.Errorf("не удалось создать пользователя: %s", firstLine(out))
		}
		return AgentSetupResult{Output: out, ScriptPath: scriptPath}, nil
	}

	// Если sudo настроен без пароля (частый случай в облачных ВМ) — без pkexec.
	if _, err := exec.LookPath("sudo"); err == nil {
		out, code, err := run("sudo", append([]string{"-n"}, args...))
		if err == nil && code == 0 {
			return AgentSetupResult{Output: out, ScriptPath: scriptPath}, nil
		}
		if err == nil && code != 0 && !isSudoAuthError(out) {
			return AgentSetupResult{}, fmt.Errorf("не удалось создать пользователя: %s", firstLine(out))
		}
	}

	if path, err := exec.LookPath("pkexec"); err == nil {
		out, code, err := run(path, args)
		if err == nil && code == 0 {
			return AgentSetupResult{Output: out, ScriptPath: scriptPath}, nil
		}
		if code == 126 || code == 127 {
			// Пользователь отменил ввод или polkit-агент недоступен.
			return AgentSetupResult{
				NeedsElevation: true,
				ScriptPath:     scriptPath,
				Command:        manual,
			}, nil
		}
		if err != nil || code != 0 {
			return AgentSetupResult{}, fmt.Errorf("не удалось создать пользователя: %s", firstLine(out))
		}
	}

	// Нет графического агента авторизации/пользователь отменил ввод.
	return AgentSetupResult{
		NeedsElevation: true,
		ScriptPath:     scriptPath,
		Command:        manual,
	}, nil
}

func boolArg(v bool) string {
	if v {
		return "1"
	}
	return "0"
}

func isSudoAuthError(out string) bool {
	low := strings.ToLower(out)
	return strings.Contains(low, "password is required") ||
		strings.Contains(low, "a terminal is required") ||
		strings.Contains(low, "no tty present") ||
		strings.Contains(low, "not allowed to execute")
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	if s == "" {
		return "неизвестная ошибка"
	}
	return s
}
