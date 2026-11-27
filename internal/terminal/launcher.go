package terminal

import "errors"

type LaunchMode string

const (
	ModeTabs  LaunchMode = "tabs"
	ModeSplit LaunchMode = "split"
)

type SessionSpec struct {
	HostID    string
	SessionID string
	Title     string
	TabColor  string
	// Script — имя скрипта ssh-launcher-cli (--run), "" — обычная сессия.
	Script string
}

// wrapperArgs — аргументы CLI-обёртки для сессии.
func wrapperArgs(s SessionSpec) []string {
	args := []string{"--connect", s.HostID, "--session", s.SessionID}
	if s.Script != "" {
		args = append(args, "--run", s.Script)
	}
	return args
}

type Launcher interface {
	Name() string
	IsAvailable() bool
	Launch(exePath string, sessions []SessionSpec, mode LaunchMode) error
}

var ErrNoTerminalFound = errors.New("no supported terminal emulator found")
