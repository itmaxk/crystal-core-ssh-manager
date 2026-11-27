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
		cmdArgs := append([]string{exePath}, wrapperArgs(s)...)

		switch l.bin {
		case "wezterm":
			args = append([]string{"start", "--"}, cmdArgs...)
		case "kitty":
			args = append([]string{"--title", s.Title}, cmdArgs...)
		case "konsole":
			args = append([]string{"--new-tab", "-e"}, cmdArgs...)
		case "gnome-terminal":
			args = append([]string{"--tab", "--title", s.Title, "--"}, cmdArgs...)
		case "alacritty":
			args = append([]string{"--title", s.Title, "-e"}, cmdArgs...)
		default:
			args = append([]string{"-e"}, cmdArgs...)
		}

		if err := exec.Command(l.bin, args...).Start(); err != nil {
			return fmt.Errorf("launch %s: %w", l.bin, err)
		}
	}
	return nil
}
