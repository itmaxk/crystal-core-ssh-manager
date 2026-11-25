//go:build !darwin

package app

import (
	_ "embed"
	goruntime "runtime"

	"fyne.io/systray"
)

// Windows-трей принимает только ICO, Linux (StatusNotifierItem) — PNG.
var (
	//go:embed icons/tray.ico
	trayIconICO []byte
	//go:embed icons/tray.png
	trayIconPNG []byte
)

const trayAvailable = true

func (a *App) startTray() {
	icon := trayIconPNG
	if goruntime.GOOS == "windows" {
		icon = trayIconICO
	}

	go func() {
		// Окно трея и его цикл сообщений должны жить в одном OS-потоке.
		goruntime.LockOSThread()
		systray.Run(func() {
			systray.SetIcon(icon)
			systray.SetTitle("Crystal-Core SSH Manager")
			systray.SetTooltip("Crystal-Core SSH Manager (Ctrl+Alt+S)")
			// Клик по иконке (Windows) открывает окно; меню — по правой кнопке.
			systray.SetOnTapped(a.showAndFocusOmnibar)

			mOpen := systray.AddMenuItem("Открыть", "Показать окно (Ctrl+Alt+S)")
			mRefresh := systray.AddMenuItem("Обновить пинг", "Проверить доступность серверов")
			systray.AddSeparator()
			mQuit := systray.AddMenuItem("Выход", "Закрыть Crystal-Core SSH Manager")

			go func() {
				for {
					select {
					case <-mOpen.ClickedCh:
						a.showAndFocusOmnibar()
					case <-mRefresh.ClickedCh:
						a.RefreshHostsStatus()
					case <-mQuit.ClickedCh:
						systray.Quit()
						a.ExitApp()
						return
					}
				}
			}()
		}, func() {})
	}()
}

func (a *App) stopTray() { systray.Quit() }
