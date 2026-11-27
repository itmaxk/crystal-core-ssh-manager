package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"ssh-launcher/internal/update"
)

// version задаётся из main (productVersion в wails.json).
var version = "0.0.0"

// SetVersion вызывается из main до запуска Wails.
func SetVersion(v string) {
	if v != "" {
		version = v
	}
}

type updateState struct {
	mu      sync.Mutex
	info    *update.Info
	staged  *update.Staged
	cancel  context.CancelFunc
	updater *update.Updater
}

// UpdateProgress — событие update:progress.
type UpdateProgress struct {
	Done  int64 `json:"done"`
	Total int64 `json:"total"`
}

// GetAppVersion возвращает версию запущенной программы.
func (a *App) GetAppVersion() string { return version }

func (a *App) updater() *update.Updater {
	a.upd.mu.Lock()
	defer a.upd.mu.Unlock()
	if a.upd.updater == nil {
		a.upd.updater = update.New(version)
	}
	return a.upd.updater
}

// CheckForUpdate спрашивает GitHub о последнем релизе.
func (a *App) CheckForUpdate() (*update.Info, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	in, err := a.updater().Check(ctx, version)
	if err != nil {
		return nil, err
	}
	a.upd.mu.Lock()
	a.upd.info = in
	if a.upd.staged != nil && a.upd.staged.Version != in.Latest {
		_ = os.RemoveAll(a.upd.staged.Dir)
		a.upd.staged = nil
	}
	a.upd.mu.Unlock()
	return in, nil
}

// DownloadUpdate скачивает и проверяет последнюю версию; прогресс — событием
// update:progress. Повторный вызов во время загрузки возвращает ошибку.
func (a *App) DownloadUpdate() error {
	a.upd.mu.Lock()
	in := a.upd.info
	if in == nil || !in.Available {
		a.upd.mu.Unlock()
		return errors.New("сначала проверьте обновления")
	}
	if a.upd.cancel != nil {
		a.upd.mu.Unlock()
		return errors.New("загрузка уже идёт")
	}
	if a.upd.staged != nil && a.upd.staged.Version == in.Latest {
		a.upd.mu.Unlock()
		return nil
	}
	parent := a.ctx
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	a.upd.cancel = cancel
	a.upd.mu.Unlock()

	defer func() {
		a.upd.mu.Lock()
		a.upd.cancel = nil
		a.upd.mu.Unlock()
		cancel()
	}()

	stageRoot, err := updateStageDir()
	if err != nil {
		return err
	}
	st, err := a.updater().Download(ctx, in, stageRoot, func(done, total int64) {
		a.emit("update:progress", UpdateProgress{Done: done, Total: total})
	})
	if err != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			return errors.New("загрузка отменена")
		}
		return err
	}
	a.upd.mu.Lock()
	a.upd.staged = st
	a.upd.mu.Unlock()
	return nil
}

// CancelUpdateDownload прерывает загрузку обновления.
func (a *App) CancelUpdateDownload() {
	a.upd.mu.Lock()
	defer a.upd.mu.Unlock()
	if a.upd.cancel != nil {
		a.upd.cancel()
	}
}

// InstallUpdate подменяет файлы скачанной версией, запускает её и завершает
// текущий процесс (туннели останавливаются, автозапускаемые поднимет новая версия).
func (a *App) InstallUpdate() error {
	a.upd.mu.Lock()
	st := a.upd.staged
	a.upd.mu.Unlock()
	if st == nil {
		return errors.New("обновление ещё не скачано")
	}
	u := a.updater()
	if err := u.Install(st); err != nil {
		return err
	}
	a.upd.mu.Lock()
	a.upd.staged = nil
	a.upd.mu.Unlock()

	exe, err := os.Executable()
	if err == nil {
		exe, err = filepath.EvalSymlinks(exe)
	}
	if err == nil {
		err = update.Restart(exe)
	}
	if err != nil {
		return errors.New("обновление установлено, но перезапустить программу не удалось — закройте её через «Выход» и запустите снова")
	}
	a.quitting.Store(true)
	_ = a.clearSecret(0)
	if a.ctx != nil && a.ctx.Value("frontend") != nil {
		runtime.Quit(a.ctx)
	}
	return nil
}

func updateStageDir() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "CrystalCoreSSHManager", "updates"), nil
}

// cleanupUpdates убирает остатки прошлого обновления (старые .exe на Windows,
// недокачанные архивы).
func (a *App) cleanupUpdates() {
	a.updater().CleanupOld()
	if dir, err := updateStageDir(); err == nil {
		_ = os.RemoveAll(dir)
	}
}
