//go:build windows

package terminal

import (
	"fmt"
	"os/exec"
	"strings"
	"syscall"
)

type WindowsTerminalLauncher struct{}

func DefaultLauncher() (Launcher, error) {
	l := &WindowsTerminalLauncher{}
	if !l.IsAvailable() {
		return nil, ErrNoTerminalFound
	}
	return l, nil
}

func (l *WindowsTerminalLauncher) Name() string {
	return "Windows Terminal"
}

func (l *WindowsTerminalLauncher) IsAvailable() bool {
	_, err := exec.LookPath("wt.exe")
	return err == nil
}

func (l *WindowsTerminalLauncher) Launch(exePath string, sessions []SessionSpec, mode LaunchMode) error {
	if len(sessions) == 0 {
		return nil
	}

	var parts []string
	for i, s := range sessions {
		subcmd := "new-tab"
		if mode == ModeSplit && i > 0 {
			if i%2 == 1 {
				subcmd = "split-pane -V"
			} else {
				subcmd = "split-pane -H"
			}
		}

		title := escapeWT(s.Title)
		var b strings.Builder
		b.WriteString(subcmd)
		if title != "" {
			fmt.Fprintf(&b, ` --title "%s"`, title)
		}
		if s.TabColor != "" {
			fmt.Fprintf(&b, ` --tabColor "%s"`, escapeWT(s.TabColor))
		}
		fmt.Fprintf(&b, ` -- "%s"`, escapeWT(exePath))
		for _, a := range wrapperArgs(s) {
			fmt.Fprintf(&b, ` "%s"`, escapeWT(a))
		}

		parts = append(parts, b.String())
	}

	rawCmd := fmt.Sprintf(`wt.exe -w 0 %s`, strings.Join(parts, " ; "))

	cmd := exec.Command("wt.exe")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CmdLine: rawCmd,
	}
	return cmd.Start()
}

func escapeWT(s string) string {
	s = strings.ReplaceAll(s, `"`, `\"`)
	return strings.ReplaceAll(s, `;`, `\;`)
}
