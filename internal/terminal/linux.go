//go:build linux

package terminal

import (
	"fmt"
	"os/exec"
)

type LinuxLauncher struct {
	bin string
}

var linuxTerminals = []string{
	"wezterm",
	"kitty",
	"konsole",
	"gnome-terminal",
	"alacritty",
	"x-terminal-emulator",
}

func DefaultLauncher() (Launcher, error) {
	for _, bin := range linuxTerminals {
		if _, err := exec.LookPath(bin); err == nil {
			return &LinuxLauncher{bin: bin}, nil
		}
	}
	return nil, ErrNoTerminalFound
}

func (l *LinuxLauncher) Name() string      { return l.bin }
func (l *LinuxLauncher) IsAvailable() bool { return l.bin != "" }

func (l *LinuxLauncher) Launch(exePath string, sessions []SessionSpec, mode LaunchMode) error {
	for _, s := range sessions {
		var args []string
		wrapperArgs := []string{exePath, "--connect", s.HostID, "--session", s.SessionID}

		switch l.bin {
		case "wezterm":
			args = append([]string{"start", "--"}, wrapperArgs...)
		case "kitty":
			args = append([]string{"--title", s.Title}, wrapperArgs...)
		case "konsole":
			args = append([]string{"--new-tab", "-e"}, wrapperArgs...)
		case "gnome-terminal":
			args = append([]string{"--tab", "--title", s.Title, "--"}, wrapperArgs...)
		case "alacritty":
			args = append([]string{"--title", s.Title, "-e"}, wrapperArgs...)
		default:
			args = append([]string{"-e"}, wrapperArgs...)
		}

		if err := exec.Command(l.bin, args...).Start(); err != nil {
			return fmt.Errorf("launch %s: %w", l.bin, err)
		}
	}
	return nil
}
