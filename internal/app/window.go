package app

import "github.com/wailsapp/wails/v2/pkg/runtime"

func (a *App) showAndFocusOmnibar() {
	if a.ctx == nil {
		return
	}
	runtime.WindowUnminimise(a.ctx)
	runtime.WindowShow(a.ctx)
	runtime.EventsEmit(a.ctx, "ui:focus_omnibar", nil)
}
