package main

import (
	"embed"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"

	"ssh-launcher/internal/app"
	"ssh-launcher/internal/cli"
	"ssh-launcher/internal/ipc"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// РЕЖИМ 1: CLI-обёртка внутри вкладки терминала (macOS / Linux).
	// Флаги разбираем только если они явно переданы: macOS при запуске из
	// Finder может добавить свои аргументы (-psn_...), и flag.Parse завершил бы процесс.
	if hasFlag("connect") {
		fs := flag.NewFlagSet("connect", flag.ExitOnError)
		connectHostID := fs.String("connect", "", "Host ID to connect via CLI wrapper")
		sessionID := fs.String("session", "", "Unique session ID for IPC tracking")
		runScript := fs.String("run", "", "Name of a predefined script to run on the host (vps-setup)")
		_ = fs.Parse(os.Args[1:])
		if *connectHostID == "" {
			fmt.Fprintln(os.Stderr, "usage: --connect <host_id> --session <session_id>")
			os.Exit(1)
		}
		os.Exit(cli.Run(*connectHostID, *sessionID, *runScript))
	}

	// РЕЖИМ 2: GUI-приложение (Single-Instance через IPC)
	if err := ipc.Send(ipc.Message{Type: ipc.MsgFocusOmnibar}); err == nil {
		return
	}

	application := app.New()

	err := wails.Run(&options.App{
		Title:            "Crystal-Core SSH Manager",
		Width:            960,
		Height:           640,
		MinWidth:         720,
		MinHeight:        500,
		Frameless:        true,
		BackgroundColour: &options.RGBA{R: 0, G: 0, B: 0, A: 0},
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		// Нативный drop файлов: WebView не отдаёт реальные пути File-объектов,
		// поэтому SFTP Dropzone получает пути через runtime.OnFileDrop.
		DragAndDrop: &options.DragAndDrop{
			EnableFileDrop:     true,
			DisableWebViewDrop: true,
		},
		OnStartup:     application.Startup,
		OnDomReady:    application.DomReady,
		OnBeforeClose: application.BeforeClose,
		OnShutdown:    application.Shutdown,
		Bind: []any{
			application,
		},
		Windows: &windows.Options{
			WebviewIsTransparent: true,
			WindowIsTranslucent:  true,
			BackdropType:         windows.Mica,
			Theme:                windows.SystemDefault,
		},
		Mac: &mac.Options{
			TitleBar:             mac.TitleBarHiddenInset(),
			WebviewIsTransparent: true,
			WindowIsTranslucent:  true,
			Appearance:           mac.DefaultAppearance,
		},
		Linux: &linux.Options{
			WindowIsTranslucent: false,
		},
	})
	if err != nil {
		log.Fatalf("wails run failed: %v", err)
	}
}

func hasFlag(name string) bool {
	for _, arg := range os.Args[1:] {
		trimmed := strings.TrimLeft(arg, "-")
		if len(trimmed) < len(arg) && (trimmed == name || strings.HasPrefix(trimmed, name+"=")) {
			return true
		}
	}
	return false
}
