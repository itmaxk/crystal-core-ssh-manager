package app

import (
	"log"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) showAndFocusOmnibar() {
	if a.ctx == nil {
		return
	}
	runtime.WindowUnminimise(a.ctx)
	runtime.WindowShow(a.ctx)
	runtime.EventsEmit(a.ctx, "ui:focus_omnibar", nil)
}

// warnf пишет предупреждение в лог Wails, а вне жизненного цикла Wails
// (тесты, ранняя инициализация) — в стандартный log: runtime.Log* в таком
// случае завершает процесс через log.Fatalf.
func (a *App) warnf(format string, args ...any) {
	if a.ctx != nil && a.ctx.Value("logger") != nil {
		runtime.LogWarningf(a.ctx, format, args...)
		return
	}
	log.Printf("WARN: "+format, args...)
}
