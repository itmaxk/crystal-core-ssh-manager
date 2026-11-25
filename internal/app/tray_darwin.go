//go:build darwin

package app

// На macOS systray требует главный поток и собственный NSApplication-цикл,
// который уже занят Wails, поэтому трей отключён (окно доступно из Dock).
func (a *App) startTray() {}

func (a *App) stopTray() {}
