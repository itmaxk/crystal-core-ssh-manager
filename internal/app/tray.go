//go:build !darwin

package app

import (
	_ "embed"
	goruntime "runtime"

	"fyne.io/systray"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// Windows-трей принимает только ICO, Linux (StatusNotifierItem) — PNG.
var (
	//go:embed icons/tray.ico
	trayIconICO []byte
	//go:embed icons/tray.png
	trayIconPNG []byte
)

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

			mOpen := systray.AddMenuItem("Открыть Omnibar", "Показать окно (Ctrl+Alt+S)")
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
						runtime.Quit(a.ctx)
						return
					}
				}
			}()
		}, func() {})
	}()
}

func (a *App) stopTray() { systray.Quit() }
