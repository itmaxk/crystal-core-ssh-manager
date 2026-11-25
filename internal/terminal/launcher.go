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
}

type Launcher interface {
	Name() string
	IsAvailable() bool
	Launch(exePath string, sessions []SessionSpec, mode LaunchMode) error
}

var ErrNoTerminalFound = errors.New("no supported terminal emulator found")
