//go:build darwin

package terminal

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type MacOSLauncher struct {
	useITerm bool
}

func DefaultLauncher() (Launcher, error) {
	_, err := os.Stat("/Applications/iTerm.app")
	return &MacOSLauncher{useITerm: err == nil}, nil
}

func (l *MacOSLauncher) Name() string {
	if l.useITerm {
		return "iTerm2"
	}
	return "Terminal.app"
}

func (l *MacOSLauncher) IsAvailable() bool { return true }

func (l *MacOSLauncher) Launch(exePath string, sessions []SessionSpec, mode LaunchMode) error {
	if len(sessions) == 0 {
		return nil
	}
	if l.useITerm {
		return l.launchITerm(exePath, sessions, mode)
	}
	return l.launchAppleTerminal(exePath, sessions)
}

func (l *MacOSLauncher) launchITerm(exePath string, sessions []SessionSpec, mode LaunchMode) error {
	var script strings.Builder
	script.WriteString("tell application \"iTerm\"\nactivate\n")
	script.WriteString("if (count of windows) = 0 then create window with default profile\n")
	script.WriteString("tell current window\n")

	for i, s := range sessions {
		shellCmd := buildUnixShellCmd(exePath, s)
		if i == 0 || mode == ModeTabs {
			script.WriteString("set newTab to (create tab with default profile)\n")
			script.WriteString("tell current session of newTab\n")
		} else {
			dir := "vertically"
			if i%2 == 0 {
				dir = "horizontally"
			}
			fmt.Fprintf(&script, "tell current session\nset splitSess to (split %s with default profile)\nend tell\n", dir)
			script.WriteString("tell splitSess\n")
		}
		if s.Title != "" {
			fmt.Fprintf(&script, "set name to \"%s\"\n", escapeAppleScript(s.Title))
		}
		fmt.Fprintf(&script, "write text \"%s\"\n", escapeAppleScript(shellCmd))
		script.WriteString("end tell\n")
	}

	script.WriteString("end tell\nend tell\n")
	return exec.Command("osascript", "-e", script.String()).Start()
}

func (l *MacOSLauncher) launchAppleTerminal(exePath string, sessions []SessionSpec) error {
	var script strings.Builder
	script.WriteString("tell application \"Terminal\"\nactivate\n")
	for _, s := range sessions {
		shellCmd := buildUnixShellCmd(exePath, s)
		fmt.Fprintf(&script, "do script \"%s\"\n", escapeAppleScript(shellCmd))
	}
	script.WriteString("end tell\n")
	return exec.Command("osascript", "-e", script.String()).Start()
}

// buildUnixShellCmd собирает команду для интерактивного shell терминала.
// Go-шный %q не является shell-quoting ($ и ` внутри "..." раскрываются),
// поэтому используем одинарные кавычки POSIX.
func buildUnixShellCmd(exePath string, s SessionSpec) string {
	return fmt.Sprintf("exec %s --connect %s --session %s", shellQuote(exePath), shellQuote(s.HostID), shellQuote(s.SessionID))
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}

func escapeAppleScript(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	return strings.ReplaceAll(s, `"`, `\"`)
}
